package main

import (
	"encoding/json"
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
	Id      int
	Reports []Report
}

func (i *Incident) latestReport() Report {
	return i.Reports[len(i.Reports)-1]
}

type Report struct {
	Hash        string
	Guid        string
	Title       string
	Link        string
	Category    string
	Pubdate     time.Time
	Updated     time.Time
	Description string
	Details     map[string]string
	Geometry    Geometry
}

func (r *Report) Id() int {
	// Take the integer at the end of the Guid to use as the Id
	s := strings.Split(r.Guid, ":")
	id, _ := strconv.Atoi(s[len(s)-1])
	return id
}
