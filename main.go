package main

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
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

		count, err := ImportJson(contents, incidents)

		if err != nil {
			fmt.Printf("<%s>\t%s\n", filename, err)
			continue
		}

		fmt.Printf("<%s>\t%d items\n", filename, count)
	}

	// For now, report on what was imported into memory
	for _, incident := range incidents {
		// Print out some details about the incident and its reports
		// (no. reports) Title
		// - <Guid> - Pubdate - Category
		// - ...
		fmt.Printf("\n<%d> (%d) %s\n", incident.Id, len(incident.Reports), incident.latestReport().Title)

		for _, report := range incident.Reports {
			fmt.Printf("%s - %s, %s\n", report.Details["updated"], report.Details["size"], report.Details["status"])
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
	report.Pubdate = f.Properties.Pubdate
	report.Description = f.Properties.Description
	report.Geometry = f.Geometry

	report.Details = make(map[string]string)
	// Make more use of the description
	// We've got a string like this:
	// ALERT LEVEL: Not Applicable<br />LOCATION: Australian Native Landscapes, Snowy Mountains Highway, Tumut<br />COUNCIL AREA: Tumut<br />STATUS: under control<br />TYPE: Tip Refuse fire<br />FIRE: Yes<br />SIZE: 0 ha<br />RESPONSIBLE AGENCY: Rural Fire Service<br />UPDATED: 5 Feb 2014 08:58
	// Split by <br />
	d := strings.Split(report.Description, "<br />")
	// This is for the KEY: Value strings
	re := regexp.MustCompile(`^([\w\s]+):\s(.*)`)
	whitespaceRe := regexp.MustCompile(`\s+`)
	for _, v := range d {
		r := re.FindAllStringSubmatch(v, -1)
		if len(r) == 1 {
			m := r[0]
			if len(m) == 3 {
				label := strings.ToLower(m[1])
				// Maybe unecessary, but I'd like to have no whitespace in the label
				label = whitespaceRe.ReplaceAllString(label, "_")
				report.Details[label] = m[2]
				// } else {
				// We're expecting 3 matches - the initial string, the key and the value
				// log.Panicf("%d - %s", len(m), m)
			}
			// } else {
			// Well, there isn't a match which means there's some random text at the end.
			// This is a chunk of text that's added onto the description
			// log.Panicf("No matches %d - %s (from %s)", len(r), r, v)
		}
	}

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
