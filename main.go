package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
  "os"
	"path/filepath"
)

func ImportFromDirectory(dir string) {

	matches, err := filepath.Glob(filepath.Join(dir, "*.json"))

	if err != nil {
		log.Fatal(err)
	}

	var incidents = make(map[string]Incident)

	for _, filename := range matches {
		contents, err := ioutil.ReadFile(filename)

		if err != nil {
      fmt.Printf("<%s>\t%s\n", filename, err)
      continue
		}

		var features FeatureCollection
		jErr := json.Unmarshal(contents, &features)
		if jErr != nil {
      fmt.Printf("<%s>\t%s\n", filename, jErr)
      continue
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
        fmt.Printf("<%s> Unknown feature type: %z\n", filename, feature.Type)
			}
			if err != nil {
        fmt.Printf("<%s> %s\n", filename, err)
			}

			incident, _ := incidentFromFeature(*feature)

			existingIncident, exists := incidents[incident.Title]
			if exists {
        // Add the incident's update to the existing one, and assign to the latest incident
        incident.IncidentUpdates = append(existingIncident.IncidentUpdates, incident.IncidentUpdates...)
      }

      incidents[incident.Title] = incident
		}
	}

  for _, incident := range incidents {
    fmt.Printf("%s\t-\t %d updates\n", incident.Title, len(incident.IncidentUpdates))
  }

  fmt.Printf("%d incidents\n", len(incidents))

}

func incidentFromFeature(f Feature) (incident Incident, err error) {
  incident = Incident{}

  incident.Title = f.Properties.Title

  incidentUpdate, _ := incidentUpdateFromFeature(f) // The 1st update
  incident.IncidentUpdates = append(incident.IncidentUpdates, incidentUpdate)

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

func main() {
  if len(os.Args) == 1 {
    log.Panic("Specify a directory to search within")
  }

  dir := os.Args[1]

  fmt.Printf("Importing from %s", dir)

  ImportFromDirectory(dir)
}

