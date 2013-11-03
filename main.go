package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
  "os"
	"path/filepath"
)

func main() {

  if len(os.Args) == 1 {
    log.Panic("Specify a directory to search within")
  }

  dir := os.Args[1]

	matches, error := filepath.Glob(filepath.Join(dir, "*.json"))

	if error != nil {
		log.Fatal(error)
	}

	var incidents = make(map[string]Incident)

	for _, filename := range matches {
		contents, error := ioutil.ReadFile(filename)

		if error != nil {
			log.Fatal(error)
		}

		var features FeatureCollection
		err := json.Unmarshal(contents, &features)
		if err != nil {
			log.Fatal(err)
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

			// Unless incidents already includes this incident, add it to the incidents slice
			_, exists := incidents[incident.Guid]

			if !exists {
				incidents[incident.Guid] = incident
			}
		}
	}

	fmt.Printf("incidents %z\n", incidents)
}

func incidentFromFeature(f Feature) (incident Incident, err error) {
  incident = Incident{}
	incident.Guid = f.Properties.Guid

	return
}
