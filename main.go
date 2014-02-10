package main

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

func ImportFromDirectory(dir string, incidents map[int]Incident) (int, error) {
	// Check if the directory exists / or if there's a permissions error there
	if _, err := os.Stat(dir); err != nil {
		return len(incidents), err
	}

	matches, globErr := filepath.Glob(filepath.Join(dir, "*.json"))

	if globErr != nil {
		return len(incidents), globErr
	}

	for _, path := range matches {
		_, iErr := ImportFromFile(path, incidents)

		if iErr != nil {
			// Continue importing if error encountered with this particular file
			continue
		}
	}

	return len(incidents), nil
}

func ImportFromFile(path string, incidents map[int]Incident) (int, error) {
	// Check if the file exists / or if there's a permissions error there
	if _, err := os.Stat(path); err != nil {
		return len(incidents), err
	}

	contents, readErr := ioutil.ReadFile(path)

	if readErr != nil {
		return len(incidents), readErr
	}

	count, iErr := ImportJson(contents, incidents)

	if iErr != nil {
		return len(incidents), iErr
	}

	return count, nil
}

func ImportFromURI(u *url.URL, incidents map[int]Incident) (int, error) {
	// Not yet...
	// URL is to an XML file, convert to GeoJSON
	// www.rfs.nsw.gov.au/feeds/majorIncidents.xml
	// check if u.Path matches json or xml

	// At the moment just hosting a JSON parsed version on Dropbox
  res, err := http.Get(u.String())
  if err != nil {
    return len(incidents), err
  }

  json, err := ioutil.ReadAll(res.Body)
  res.Body.Close()
  if err != nil {
    return len(incidents), err
  }

	count, iErr := ImportJson(json, incidents)
	if iErr != nil {
		return len(incidents), iErr
	}

	return count, nil
}

func ImportJson(data []byte, incidents map[int]Incident) (int, error) {
	var features FeatureCollection
	jErr := json.Unmarshal(data, &features)
	if jErr != nil {
		return len(incidents), jErr
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
	// Pull expected details into the struct as fields

	// TODO

	// Updated details should be of type time
	// Pull updated detail into the struct since it's time.Time
	loc, _ := time.LoadLocation("Australia/Sydney")
	updatedFormat := "2 Jan 2006 15:04"
	report.Updated, _ = time.ParseInLocation(updatedFormat, details["updated"], loc)

	return
}

func main() {
	if len(os.Args) == 1 {
		log.Panic("Specify a URL or directory to import from")
	}

	loc := os.Args[1]

	fmt.Printf("Importing from %s\n", loc)

	// Initialise our in-memory data store
	var incidents = make(map[int]Incident)

	var err error
	var count int
	// Argument could be URL or path
	if u, urlErr := url.Parse(loc); urlErr == nil {
		if u.IsAbs() {
			count, err = ImportFromURI(u, incidents)
		} else {
			count, err = ImportFromDirectory(loc, incidents)
		}
	} else {
		count, err = ImportFromDirectory(loc, incidents)
	}

	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Imported %d incidents from %s\n", count, loc)

	// For now, report on what was imported into memory
	for _, incident := range incidents {
		// Print out some details about the incident and its reports
		// (no. reports) Title
		// - <Guid> - Pubdate - Category
		// - ...
		fmt.Printf("\n<%d> (%d) %s\n", incident.Id, len(incident.Reports), incident.latestReport().Title)
	}
}
