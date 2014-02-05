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
				// Get the current report
				current := existingIncident.Reports[len(existingIncident.Reports)-1]
				latest := incident.Reports[len(incident.Reports)-1]
				// See if the current report for the existing incident has the same hash of data as this latest report
				if current.Hash == latest.Hash {
					continue // Report hasn't changed, so move on
				}

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

		for k, report := range incident.Reports {
			pubdateChanged := false
			updatedChanged := false
			hashChanged := false
			if k > 0 {
				prevReport := incident.Reports[k-1]
				// If this report's Pubdate, updated or hash differ from the previous report, I want to display this
				// If they always only change at the same time, then we have a consistent way of knowing if there's an update or not
				// Otherwise, the data is changing a bit inconsistently.
				pubdateChanged = prevReport.Pubdate != report.Pubdate
				updatedChanged = prevReport.Details["updated"] != report.Details["updated"]
				hashChanged = prevReport.Hash != report.Hash
			}
			if pubdateChanged || updatedChanged || hashChanged {
				fmt.Printf("Pubdate:\t%t\tUpdated:\t%t\tHash:\t%t\t\n", pubdateChanged, updatedChanged, hashChanged)
			}
			fmt.Printf(" - <%s> %s - %s - %x\n", report.Guid, report.Pubdate, report.Details["updated"], report.Hash[0:7])
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

	// Generate hash of json representation of feature
	s, _ := json.Marshal(f)
	h := sha1.New()
	h.Write([]byte(s))
	report.Hash = fmt.Sprintf("%x", h)

	report.Guid = f.Properties.Guid
	report.Category = f.Properties.Category
	report.Pubdate = f.Properties.Pubdate
	report.Description = f.Properties.Description

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
