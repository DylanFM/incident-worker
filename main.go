package main

import (
	"crypto/sha1"
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"fmt"
	_ "github.com/lib/pq"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
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

	// For the incidents in the unmarshalled XML feed
	var incidents []Incident

	// Feed each item to a worker which turns the items into incident/report structs
	for _, item := range feed.Channel.Items {
		incidentChan := make(chan Incident)

		go func(item Item) {
			incident, _ := incidentFromItem(item)

			incident.Import()

			incidentChan <- incident
		}(item)

		incident := <-incidentChan

		incidents = append(incidents, incident)
	}

	// Update current incidents to the latest import
	err = UpdateCurrentIncidents(incidents)
	if err != nil {
		return err
	}

	return nil
}

func UpdateCurrentIncidents(incidents []Incident) error {
	// We have a collection of incidents
	// These incidents are now considered "current"
	// In the database we've got a number of incidents marked as current too

	// Get the IDs of the new incidents

	args := make([]interface{}, len(incidents))
	for i, incident := range incidents {
		args[i] = incident.UUID
	}

	// Having trouble building the variable length IN clause for this query
	ins := strings.Split(strings.Repeat("$", len(args)), "")
	for i, _ := range ins {
		ins[i] = fmt.Sprintf("$%d", i+1)
	}
	// We've got a slice of ["$1", "$2" ...]

	// Set all current incidents who aren't in this collection of incidents to not current
	q := fmt.Sprintf(`UPDATE incidents SET current = false WHERE current = true AND uuid NOT IN (%s)`, strings.Join(ins, ","))
	stmt, err := db.Prepare(q)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(args...)
	if err != nil {
		return err
	}

	// Now the only incidents marked as current in the database will be from this update

	return nil
}

func GetNumCurrentIncidents() (int, error) {
	stmt, err := db.Prepare(`SELECT COUNT(*) FROM incidents WHERE current = true`)
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
	incident.FirstSeen = report.Pubdate // Used when setting the initial tstzrange

	return
}

// Takes an array of GeoRSS polygon strings and returns a MultiPolygon representation for insertion into the database
func toMultiPolygon(shapes []string) string {
	str := "MULTIPOLYGON"
	pols := make([]string, len(shapes))

	// For each member string
	for n, v := range shapes {
		s := strings.Split(v, " ") // Split by space
		var pts [][]string         // To hold the points

		// Build a collection of points, grouped by pair
		for i, p := range s {
			if i%2 == 0 {
				// Make a new slice for this item and the next
				pt := make([]string, 2)
				pt[1] = p
				pts = append(pts, pt)
			} else {
				// Find the most recent slice and add this item
				pts[len(pts)-1][0] = p
			}
		}

		// For holding the each pt above as a string joined by a space
		strs := make([]string, len(pts))

		// For each pair of members
		for i, pt := range pts {
			// Join by a space as a string
			strs[i] = strings.Join(pt, " ")
		}

		// Join by commas and surround by parentheses
		pols[n] = "((" + strings.Join(strs, ", ") + "))"
	}

	// Join all by commas and surround by parentheses
	str = str + "(" + strings.Join(pols, ",") + ")"

	return str
}

// Converts a string representation of a coordinate to a GeoJSON representation
func toPoint(s string) string {
	// s is in form of "-33.6097 150.0216"
	pt := strings.Split(s, " ")

	return "POINT(" + pt[1] + " " + pt[0] + ")"
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

	report.Points = i.Points[0] // NOTE I'm using the 1st item here, assuming we'll only have 1 point per-item

	report.Geometry = i.Polygons

	// Pubdate should be of type time
	pubdateFormat := "Mon, 2 Jan 2006 15:04:05 GMT"
	report.Pubdate, _ = time.Parse(pubdateFormat, i.Pubdate)

	details, err := report.parsedDescription()
	// Pull expected details into the struct as fields

	loc, _ := time.LoadLocation("Australia/Sydney")
	updatedFormat := "2 Jan 2006 15:04"
	report.Updated, _ = time.ParseInLocation(updatedFormat, details["updated"], loc) // Convert to time

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
	stCiCount, _ := GetNumCurrentIncidents()
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
	enCiCount, _ := GetNumCurrentIncidents()
	enRpCount, _ := GetNumReports()

	fmt.Printf("%d new incidents, %d total\n", enInCount-stInCount, enInCount)
	fmt.Printf("%d new reports, %d total\n", enRpCount-stRpCount, enRpCount)
	fmt.Printf("%d current incidents, %d change\n", enCiCount, enCiCount-stCiCount)
}
