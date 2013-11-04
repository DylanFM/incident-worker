package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
  "os"
	"path/filepath"
  "regexp"
)

func main() {

  if len(os.Args) == 1 {
    log.Panic("Specify a directory to search within")
  }

  dir := os.Args[1]

	matches, err := filepath.Glob(filepath.Join(dir, "*.json"))

	if err != nil {
		log.Fatal(err)
	}

	var incidents = make(map[string]Incident)

	for _, filename := range matches {
		contents, err := ioutil.ReadFile(filename)

    fmt.Printf("Reading file %z\n", filename)

		if err != nil {
			log.Fatal(err)
		}

		var features FeatureCollection
		jErr := json.Unmarshal(contents, &features)
		if jErr != nil {
			log.Fatal(jErr)
		}

		for i := range features.Features {
			feature := &features.Features[i]

			switch feature.Geometry.Type {
			case "Point":
				err = json.Unmarshal(feature.Geometry.Coordinates, &feature.Geometry.Point.Coordinates)
			case "LineString":
				err = json.Unmarshal(feature.Geometry.Coordinates, &feature.Geometry.Line.Points)
			case "Polygon":
				err = json.Unmarshal(feature.Geometry.Coordinates, &feature.Geometry.Polygon.Lines)
			default:
				log.Panicf("Unknown feature type: %z\n", feature.Type)
			}
			if err != nil {
				log.Fatal(err)
			}

			incident, _ := incidentFromFeature(*feature)

			existingIncident, exists := incidents[incident.Id]
			if exists {
        fmt.Printf("Incident %z exists, appending updates\n", incident.Id)

        // Add the incident's update to the existing one
        existingIncident.IncidentUpdates = append(existingIncident.IncidentUpdates, incident.IncidentUpdates...)
      } else {
        fmt.Printf("Incident %z is new\n", incident.Id)

        // Add the incident with update to the slice
				incidents[incident.Id] = incident
			}
      fmt.Printf("num incidents %z\n", len(incidents))
		}
	}

	fmt.Printf("incidents %z\n", incidents)
}

func incidentFromFeature(f Feature) (incident Incident, err error) {
  incident = Incident{}

  incident.Link  = f.Properties.Link
  incident.Title = f.Properties.Title

  incidentUpdate, _ := incidentUpdateFromFeature(f) // The 1st update
  incident.IncidentUpdates = append(incident.IncidentUpdates, incidentUpdate)

  // Get the Id from the link
  re := regexp.MustCompile("=\\d+$")
  id := re.FindString(incident.Link)

  if len(id) == 0 {
    log.Panicf("Can't get id from %z\n", incident.Link)
  }

  incident.Id = id

	return
}

func incidentUpdateFromFeature(f Feature) (incidentUpdate IncidentUpdate, err error) {
  incidentUpdate = IncidentUpdate{}

  incidentUpdate.Guid        = f.Properties.Guid
  incidentUpdate.Category    = f.Properties.Category
  incidentUpdate.Pubdate     = f.Properties.Pubdate
  incidentUpdate.Description = f.Properties.Description

  return
}

