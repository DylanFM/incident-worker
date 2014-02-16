package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// {
//   "type": "FeatureCollection",
//   "crs": { "type": "name", "properties": { "name": "urn:ogc:def:crs:OGC:1.3:CRS84" } },
//   "features": [{
//     "type": "Feature",
//     "properties": {
//       "title": "Turners Road",
//       "link": "http:\/\/www.rfs.nsw.gov.au\/dsp_content.cfm?cat_id=683",
//       "category": "Advice",
//       "guid": "tag:www.rfs.nsw.gov.au,2013-11-02:80707",
//       "guid_isPermaLink": "false",
//       "pubDate": "2013\/10\/31 00:00:00+00",
//       "description": "..."
//     },
//     "geometry": {
//       "type": "Polygon",
//       "coordinates": [ [ [ 151.4124, -32.9768 ] ] ]
//     }
//   }]
// }

type Point struct {
	Coordinates []float64
}

type Line struct {
	Points [][]float64
}

type Polygon struct {
	Lines [][][]float64
}

type Geometry struct {
	Type        string
	Coordinates json.RawMessage
	Point       Point
	Line        Line
	Polygon     Polygon
}

type Properties struct {
	Title       string
	Link        string
	Category    string
	Guid        string
	Pubdate     string
	Description string
}

type Feature struct {
	Type       string
	Properties Properties
	Geometry   Geometry
}

type CrsProperties struct {
	name string
}

type Crs struct {
	Type       string
	Properties CrsProperties
}

type FeatureCollection struct {
	Type     string
	Crs      Crs
	Features []Feature
}

// Now non-Geojson marshalling stuff...

type Incident struct {
	UUID      string
	RFSId     int
	Current   bool
	CreatedAt time.Time
	UpdatedAt time.Time

	Reports []Report
}

func (i *Incident) latestReport() Report {
	return i.Reports[len(i.Reports)-1]
}

func (i *Incident) Import() error {
	uuid, err := GetIncidentUUIDForRFSId(i.RFSId)
	if err != nil && err != sql.ErrNoRows {
		// There's an error and it's not that there is no record
		return err
	}

	if uuid != "" {
		i.UUID = uuid

		// We've got a report for this incident, so ensure that it's set to current
		err = i.SetCurrent()
		if err != nil {
			return err
		}
	} else {
		// The incident will automatically be set to current in the DB
		// Because this is created in reponse to a report, that's correct
		err = i.Insert()
		if err != nil {
			return err
		}
	}

	// get id from reports where hash = incident.latestReport().Hash

	// if row doesnt exist {
	//   insert report into reports and ensure references incident
	// }

	return nil
}

// Sets the incident's current column to true if it isn't already
func (i *Incident) SetCurrent() error {
	stmt, err := db.Prepare(`UPDATE incidents SET current = true WHERE uuid = $1 AND current = false`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(i.UUID)
	if err != nil {
		return err
	}

	i.Current = true

	return nil
}

// Inserts the incident into the database
func (i *Incident) Insert() error {
	if i.UUID != "" {
		return fmt.Errorf("Attempting to insert incident that already has a UUID, %s", i.UUID)
	}
	stmt, err := db.Prepare(`INSERT INTO incidents(rfs_id) VALUES($1) RETURNING uuid`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	err = stmt.QueryRow(i.RFSId).Scan(&i.UUID)
	if err != nil {
		return err
	}
	return nil
}

type Report struct {
	UUID              string
	IncidentUUID      string
	Hash              string
	Guid              string
	Title             string
	Link              string
	Category          string
	Pubdate           time.Time
	Description       string
	Updated           time.Time
	AlertLevel        string
	Location          string
	CouncilArea       string
	Status            string
	FireType          string
	Fire              bool
	Size              string
	ResponsibleAgency string
	Extra             string
	Geometry          Geometry
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (r *Report) Id() int {
	// Take the integer at the end of the Guid to use as the Id
	s := strings.Split(r.Guid, ":")
	id, _ := strconv.Atoi(s[len(s)-1])
	return id
}

// Make more use of the description
// We've got a string like this:
// ALERT LEVEL: Not Applicable<br />LOCATION: Australian Native Landscapes, Snowy Mountains Highway, Tumut<br />COUNCIL AREA: Tumut<br />STATUS: under control<br />TYPE: Tip Refuse fire<br />FIRE: Yes<br />SIZE: 0 ha<br />RESPONSIBLE AGENCY: Rural Fire Service<br />UPDATED: 5 Feb 2014 08:58
func (r *Report) parsedDescription() (map[string]string, error) {
	details := make(map[string]string)

	// Split by <br />
	d := strings.Split(r.Description, "<br />")
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
				details[label] = m[2]
			}
		} else {
			// Well, there isn't a match which means there's some random text at the end.
			// This is a chunk of text that's added onto the description
			// return nil, fmt.Errorf("No matches %d - %s (from %s)", len(r), r, v)
			// Store as extra
			details["extra"] = v
		}
	}

	return details, nil
}
