package main

import "encoding/json"

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
  Type string
  Coordinates json.RawMessage
  Point Point
  Line Line
  Polygon Polygon
}

type Properties struct {
  Title string
  Link string
  Category string
  Guid string
  Pubdate string
  Description string
}

type Feature struct {
  Type string
  Properties Properties
  Geometry Geometry
}

type CrsProperties struct {
  name string
}

type Crs struct {
  Type string
  Properties CrsProperties
}

type FeatureCollection struct {
  Type string
  Crs Crs
  Features []Feature
}

// Now non-Geojson marshalling stuff...

type Incident struct {
  Guid string
}
