package main

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"
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

		_, iErr := ImportJson(contents, incidents)

		if iErr != nil {
			fmt.Printf("<%s>\t%s\n", filename, iErr)
			continue
		}
	}

	// For now, report on what was imported into memory
	for _, incident := range incidents {
		// Print out some details about the incident and its reports
		// (no. reports) Title
		// - <Guid> - Pubdate - Category
		// - ...
		fmt.Printf("\n<%d> (%d) %s\n", incident.Id, len(incident.Reports), incident.latestReport().Title)

		for _, report := range incident.Reports {
			fmt.Printf("%s\n", report.Updated.Format("15:04 Mon Jan 2 2006"))
		}
	}

	return
}

func ImportJson(data []byte, incidents map[int]Incident) (int, error) {

	var features FeatureCollection
	jErr := json.Unmarshal(data, &features)
	if jErr != nil {
		return 0, jErr
	}

	for i := range features.Features {
		var err error
		feature := &features.Features[i]

		switch feature.Geometry.Type {
		case "Point":
			err = json.Unmarshal(feature.Geometry.Coordinates, &feature.Geometry.Point.Coordinates)
		case "LineString":
			err = json.Unmarshal(feature.Geometry.Coordinates, &feature.Geometry.Line.Points)
		case "Polygon":
			err = json.Unmarshal(feature.Geometry.Coordinates, &feature.Geometry.Polygon.Lines)
		default:
			fmt.Printf("Unknown feature type: %z\n", feature.Type)
		}
		if err != nil {
			fmt.Println(err)
		}

		incident, _ := incidentFromFeature(*feature)

		existingIncident, exists := incidents[incident.Id]
		if exists {
			// See if the current report for the existing incident has the same hash of data as this latest report
			if existingIncident.latestReport().Hash == incident.latestReport().Hash {
				continue // Report hasn't changed, so move on
			}

			// Add the incident's report to the existing one, and assign to the latest incident
			incident.Reports = append(existingIncident.Reports, incident.Reports...)
		}

		incidents[incident.Id] = incident
	}

	return len(incidents), nil
}

func incidentFromFeature(f Feature) (incident Incident, err error) {
	incident = Incident{}

	report, _ := reportFromFeature(f) // The 1st report
	incident.Reports = append(incident.Reports, report)

	incident.Id = report.Id()

	return
}

func reportFromFeature(f Feature) (report Report, err error) {
	report = Report{}

	// Generate hash of json representation of feature
	s, _ := json.Marshal(f)
	h := sha1.New()
	h.Write([]byte(s))
	report.Hash = fmt.Sprintf("%x", h)

	report.Guid = f.Properties.Guid
	report.Title = f.Properties.Title
	report.Link = f.Properties.Link
	report.Category = f.Properties.Category
	report.Description = f.Properties.Description
	report.Geometry = f.Geometry
	// Pubdate should be of type time
	pubdateFormat := "2006/01/02 15:04:05-07"
	report.Pubdate, _ = time.Parse(pubdateFormat, f.Properties.Pubdate)

	details, err := report.parsedDescription()

	fmt.Println(details)

	// Updated details should be of type time
	// Pull updated detail into the struct since it's time.Time
	loc, _ := time.LoadLocation("Australia/Sydney")
	updatedFormat := "2 Jan 2006 15:04"
	report.Updated, _ = time.ParseInLocation(updatedFormat, details["updated"], loc)

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
