package main

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func ImportFromDirectory(dir string) {

	matches, err := filepath.Glob(filepath.Join(dir, "*.json"))

	if err != nil {
		log.Fatal(err)
	}

	var incidents = make(map[int]Incident)

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

			existingIncident, exists := incidents[incident.Id]
			if exists {
				// Add the incident's report to the existing one, and assign to the latest incident
				incident.Reports = append(existingIncident.Reports, incident.Reports...)
			}

			incidents[incident.Id] = incident
		}
	}

	for _, incident := range incidents {
		// Print out some details about the incident and its reports
		// (no. reports) Title
		// - <Guid> - Pubdate - Category
		// - ...
		fmt.Printf("\n<%d> (%d) %s\n", incident.Id, len(incident.Reports), incident.Title)

		for _, report := range incident.Reports {
			fmt.Printf(" - <%s> %s - %x - %s\n", report.Guid, report.Pubdate, report.Hash[0:7], report.Description)
		}
	}

	fmt.Printf("%d incidents\n", len(incidents))
}

func incidentFromFeature(f Feature) (incident Incident, err error) {
	incident = Incident{}

	incident.Title = f.Properties.Title

	report, _ := reportFromFeature(f) // The 1st report
	incident.Reports = append(incident.Reports, report)

	// Take the integer at the end of the Guid to use as the Id
	s := strings.Split(report.Guid, ":")
	incident.Id, _ = strconv.Atoi(s[len(s)-1])

	return
}

func reportFromFeature(f Feature) (report Report, err error) {
	report = Report{}

	report.Guid = f.Properties.Guid
	report.Category = f.Properties.Category
	report.Pubdate = f.Properties.Pubdate
	report.Description = f.Properties.Description
	// Generate hash of json representation of feature
	s, _ := json.Marshal(f)
	h := sha1.New()
	h.Write([]byte(s))
	report.Hash = fmt.Sprintf("%x", h)

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
