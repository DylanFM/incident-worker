package main

import (
	"bytes"
	"crypto/sha1"
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/lib/pq"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

var db *sql.DB // Global for database connection

func ImportFromDirectory(dir string) (int, error) {
	// Check if the directory exists / or if there's a permissions error there
	if _, err := os.Stat(dir); err != nil {
		return 0, err
	}

	matches, globErr := filepath.Glob(filepath.Join(dir, "*.json"))

	if globErr != nil {
		return 0, globErr
	}

	for _, path := range matches {
		_, iErr := ImportFromFile(path)

		if iErr != nil {
			// Continue importing if error encountered with this particular file
			continue
		}
	}

	// returning 1 which is silly... should return how many imported?
	return 1, nil
}

func ImportFromFile(path string) (int, error) {
	// Check if the file exists / or if there's a permissions error there
	if _, err := os.Stat(path); err != nil {
		return 0, err
	}

	contents, readErr := ioutil.ReadFile(path)

	if readErr != nil {
		return 0, readErr
	}

	count, iErr := ImportJson(contents)

	if iErr != nil {
		return 0, iErr
	}

	return count, nil
}

func ImportFromURI(u *url.URL) (int, error) {
	res, err := http.Get(u.String())
	if err != nil {
		return 0, err
	}
	contents, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return 0, err
	}

	var json []byte

	// Contents may be JSON or XMl
	// If XML, run through the ogre conversion service so we have geojson
	if string(contents[0]) == "<" {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile("upload", "upload.xml")
		if err != nil {
			return 0, err
		}

		_, err = io.Copy(part, bytes.NewReader(contents))
		if err != nil {
			return 0, err
		}

		err = writer.Close()
		if err != nil {
			return 0, err
		}

		req, err := http.NewRequest("POST", "http://ogre.adc4gis.com/convert", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		// dump, err := httputil.DumpRequest(req, true)
		// if err != nil {
		// 	return 0, err
		// }
		// fmt.Println(string(dump))

		client := &http.Client{}

		res, err := client.Do(req)
		if err != nil {
			return 0, err
		}

		defer res.Body.Close()
		json, err = ioutil.ReadAll(res.Body)

		// dump, err = httputil.DumpResponse(res, true)
		// if err != nil {
		// 	return 0, err
		// }
		// fmt.Println(string(dump))

		if err != nil {
			return 0, err
		}
	} else {
		json = contents
	}

	count, iErr := ImportJson(json)
	if iErr != nil {
		return 0, iErr
	}

	return count, nil
}

func ImportJson(data []byte) (int, error) {
	var features FeatureCollection
	err := json.Unmarshal(data, &features)
	if err != nil {
		return 0, err
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
			err = fmt.Errorf("Unknown feature type: %z\n", feature.Type)
		}
		if err != nil {
			return 0, err
		}

		incident, err := incidentFromFeature(*feature)
		if err != nil {
			return 0, err
		}

		err = incident.Import()
		if err != nil {
			return 0, err
		}
	}

	// TODO Any incidents that were previously marked as current but weren't included in this import should have current set to false

	return len(features.Features), nil
}

// This function takes an integer that should be an RFS Id for an Incident
// If the incident exists in the database, it will return its UUID
func GetIncidentUUIDForRFSId(id int) (string, error) {
	stmt, err := db.Prepare(`SELECT uuid FROM incidents WHERE rfs_id = $1`)
	if err != nil {
		return "", err
	}
	defer stmt.Close()

	var uuid string
	err = stmt.QueryRow(id).Scan(&uuid)
	if err != nil {
		// err very well may be sql.ErrNoRows which says that no rows matched the rfs_id
		return "", err
	}
	// We have the uuid of an existing incident
	return uuid, nil
}

// Takes a string which should be a hash for a report
// If the hash exists, we return the matching row's UUID
func GetReportUUIDForHash(hash string) (string, error) {
	stmt, err := db.Prepare(`SELECT uuid FROM reports WHERE hash = $1`)
	if err != nil {
		return "", err
	}
	defer stmt.Close()

	var uuid string
	err = stmt.QueryRow(hash).Scan(&uuid)
	if err != nil {
		// err very well may be sql.ErrNoRows which says that no rows matched the hash
		return "", err
	}
	// We have the uuid of an existing report
	return uuid, nil
}

func incidentFromFeature(f Feature) (incident Incident, err error) {
	incident = Incident{}

	report, _ := reportFromFeature(f) // The 1st report
	incident.Reports = append(incident.Reports, report)

	incident.RFSId = report.Id()

	return
}

func reportFromFeature(f Feature) (report Report, err error) {
	report = Report{}

	// Generate hash of json representation of feature
	s, _ := json.Marshal(f)
	h := sha1.New()
	h.Write([]byte(s))
	report.Hash = fmt.Sprintf("%x", h.Sum(nil))

	report.Guid = f.Properties.Guid
	report.Title = f.Properties.Title
	report.Link = f.Properties.Link
	report.Category = f.Properties.Category
	report.Description = f.Properties.Description
	report.Geometry = f.Geometry
	// Pubdate should be of type time
	pubdateFormat := "2006/01/02 15:04:05-07"
	pubdateAest, _ := time.Parse(pubdateFormat, f.Properties.Pubdate)
	report.Pubdate = pubdateAest.UTC()

	details, err := report.parsedDescription()
	// Pull expected details into the struct as fields

	loc, _ := time.LoadLocation("Australia/Sydney")
	updatedFormat := "2 Jan 2006 15:04"
	updatedAest, _ := time.ParseInLocation(updatedFormat, details["updated"], loc) // Convert to time
	report.Updated = updatedAest.UTC()

	report.AlertLevel = details["alert_level"]
	report.Location = details["location"]
	report.CouncilArea = details["council_area"]
	report.Status = details["status"]
	report.FireType = details["type"] // type is reserved, so use fire_type
	report.Fire = details["fire"] == "Yes"
	report.Size = details["size"]
	report.ResponsibleAgency = details["responsible_agency"]
	report.Extra = details["extra"]

	return
}

func main() {
	if len(os.Args) == 1 {
		log.Panic("Specify a URL or directory to import from")
	}

	loc := os.Args[1]

	fmt.Printf("Importing from %s\n", loc)

	// Open up a connection to the DB (well, just get the pool going)
	var err error
	db, err = sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	var count int
	// Argument could be URL or path
	if u, urlErr := url.Parse(loc); urlErr == nil {
		if u.IsAbs() {
			count, err = ImportFromURI(u)
		} else {
			count, err = ImportFromDirectory(loc)
		}
	} else {
		count, err = ImportFromDirectory(loc)
	}

	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Imported %d incidents from %s\n", count, loc)
}
