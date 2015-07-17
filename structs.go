package main

import (
	"database/sql"
	"fmt"
	"github.com/paulmach/go.geojson"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Incident struct {
	UUID      string
	RFSId     int
	Current   bool
	FirstSeen time.Time
	CreatedAt time.Time
	UpdatedAt time.Time

	Reports []Report
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

	r := i.Reports[len(i.Reports)-1] // Get report that gave us this incident
	r.IncidentUUID = i.UUID          // Update this on the report

	// See if we have this report already
	_, err = GetReportUUIDForHash(r.Hash)
	if err != nil {
		if err != sql.ErrNoRows {
			// The error isn't that we don't have a record
			return err
		}
		// We don't have this report
		err = r.Insert()
		if err != nil {
			return err
		}
		// Possibly set this report as the latest
		err = r.SetPubdateAsIncidentCurrentFromUpper()
		if err != nil {
			return err
		}
	}

	return nil
}

// Sets the incident's current column to true if it isn't already
func (i *Incident) SetCurrent() error {
	stmt, err := db.Prepare(`UPDATE incidents SET current = true, updated_at = (NOW() AT TIME ZONE 'UTC') WHERE uuid = $1 AND current = false`)
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
	stmt, err := db.Prepare(`INSERT INTO incidents(rfs_id, current_from) VALUES($1, $2) RETURNING uuid`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	// A postgres tstzrange which holds when the incident was first and last seen
	// For now, we're creating an incident, so include its FirstSeen for both values
	firstSeenStr := i.FirstSeen.UTC().Format(time.RFC3339)
	currentRange := fmt.Sprintf("[%s,%s]", firstSeenStr, firstSeenStr)

	err = stmt.QueryRow(i.RFSId, currentRange).Scan(&i.UUID)
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
	Points            string // Just the 1st point... maybe we add support for multiple points at some point
	Geometry          *geojson.Geometry
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
				details[label] = strings.Trim(m[2], " ")
			}
		} else {
			// Well, there isn't a match which means there's some random text at the end.
			// This is a chunk of text that's added onto the description
			// return nil, fmt.Errorf("No matches %d - %s (from %s)", len(r), r, v)
			// Store as extra
			details["extra"] = v
		}
	}

	// If the updated at part has an <a> tag in it, take the first section and drop the rest
	if u, exists := details["updated"]; exists {
		if strings.Contains(u, "<a") {
			details["updated"] = strings.Trim(strings.Split(u, "<a")[0], " ")
		}
	}

	return details, nil
}

// Inserts the report into the database
func (r *Report) Insert() error {
	if r.UUID != "" {
		return fmt.Errorf("Attempting to insert report that already has a UUID, %s", r.UUID)
	}

	// Turn the geometry into a JSON string for Postgis
	geom, err := r.Geometry.MarshalJSON()
	if err != nil {
		return err
	}

	stmt, err := db.Prepare(`INSERT INTO
    reports(incident_uuid, hash, guid, title, link, category, pubdate, description, updated, alert_level, location, council_area, status, fire_type, fire, size, responsible_agency, extra, geometry)
    VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, ST_SetSRID(ST_GeomFromGeoJSON($19), 4326))
    RETURNING uuid`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	err = stmt.QueryRow(r.IncidentUUID, r.Hash, r.Guid, r.Title, r.Link, r.Category, r.Pubdate.UTC().Format(time.RFC3339), r.Description, r.Updated.UTC().Format(time.RFC3339), r.AlertLevel, r.Location, r.CouncilArea, r.Status, r.FireType, r.Fire, r.Size, r.ResponsibleAgency, r.Extra, geom).Scan(&r.UUID)
	if err != nil {
		return err
	}
	return nil
}

// If this is the latest report for an incident, update the incident's current_from column with this report's pubdate
func (r *Report) SetPubdateAsIncidentCurrentFromUpper() error {
	// Update the incident's current_from upper bound with this pubdate if it's greater than the current upper bound
	stmt, err := db.Prepare(`UPDATE incidents
    SET current_from = tstzrange(lower(current_from), $1)
    WHERE uuid = $2 AND upper(current_from) < $3::timestamptz`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(r.Pubdate.UTC().Format(time.RFC3339), r.IncidentUUID, r.Pubdate.UTC().Format(time.RFC3339))
	if err != nil {
		return err
	}
	return nil
}
