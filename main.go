package main

import (
	"crypto/sha1"
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/kpawlik/geojson"
	_ "github.com/lib/pq"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

var db *sql.DB // Global for database connection

func ImportFromFile(path string) error {
	// Check if the file exists / or if there's a permissions error there
	if _, err := os.Stat(path); err != nil {
		return err
	}

	contents, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	err = ImportXml(contents)
	if err != nil {
		return err
	}

	return nil
}

func ImportFromURI(u *url.URL) error {
	res, err := http.Get(u.String())
	if err != nil {
		return err
	}

	contents, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return err
	}

	err = ImportXml(contents)
	if err != nil {
		return err
	}

	return nil
}

// Takes a GeoRSS feed and imports features and reports from the XML
func ImportXml(data []byte) error {

	// We've got an XML file
	// The file contains some metadata and a collection of items
	// We want to get each of the items into an array
	var feed Feed
	err := xml.Unmarshal(data, &feed)
	if err != nil {
		return err
	}

	// Feed each item to a worker which turns the items into incident/report structs
	for _, item := range feed.Channel.Items {
		incidentChan := make(chan Incident)

		go func(item Item) {
			incident, _ := incidentFromItem(item)

			incident.Import()

			incidentChan <- incident
		}(item)

		<-incidentChan
	}

	return nil
}

func GetNumIncidents() (int, error) {
	stmt, err := db.Prepare(`SELECT COUNT(*) FROM incidents`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	var count int
	err = stmt.QueryRow().Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func GetNumReports() (int, error) {
	stmt, err := db.Prepare(`SELECT COUNT(*) FROM reports`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	var count int
	err = stmt.QueryRow().Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
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

func incidentFromItem(i Item) (incident Incident, err error) {
	incident = Incident{}

	report, _ := reportFromItem(i) // The 1st report
	incident.Reports = append(incident.Reports, report)

	incident.RFSId = report.Id()

	return
}

func toGeoJsonPoints(s string) (string, error) {
	// p is in form of "-33.6097 150.0216"
	ll := strings.Split(s, " ")

	flLat, _ := strconv.ParseFloat(ll[0], 64)
	flLng, _ := strconv.ParseFloat(ll[1], 64)

	lat := geojson.Coord(flLat)
	lng := geojson.Coord(flLng)

	c := geojson.Coordinate{lng, lat}

	p := geojson.NewPoint(c)

	gj, _ := json.Marshal(p)

	return string(gj), nil
}

func reportFromItem(i Item) (report Report, err error) {
	report = Report{}

	// Generate hash of json representation of item
	s, _ := json.Marshal(i)
	h := sha1.New()
	h.Write([]byte(s))
	report.Hash = fmt.Sprintf("%x", h.Sum(nil))

	report.Guid = i.Guid
	report.Title = i.Title
	report.Link = i.Link
	report.Category = i.Category
	report.Description = i.Description

	// Geometries need to be converted into GeoJSON representations for easy PostGIS insertion
	report.Points, _ = toGeoJsonPoints(i.Points[0]) // NOTE I'm using the 1st item here, assuming we'll only have 1 point per-item

	// report.Polygons = i.Polygons

	// Pubdate should be of type time
	pubdateFormat := "2006/01/02 15:04:05-07"
	pubdateAest, _ := time.Parse(pubdateFormat, i.Pubdate)
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
		log.Panic("Specify a URL or file to import from")
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

	// Get the start count for incidents and reports
	stInCount, _ := GetNumIncidents()
	stRpCount, _ := GetNumReports()

	// Argument could be URL or path
	if u, urlErr := url.Parse(loc); urlErr == nil {
		if u.IsAbs() {
			err = ImportFromURI(u)
		} else {
			err = ImportFromFile(loc)
		}
	}

	if err != nil {
		log.Fatal(err)
	}

	// Get the start count for incidents and reports
	enInCount, _ := GetNumIncidents()
	enRpCount, _ := GetNumReports()

	fmt.Printf("%d new incidents, %d total\n", enInCount-stInCount, enInCount)
	fmt.Printf("%d new reports, %d total\n", enRpCount-stRpCount, enRpCount)
}
