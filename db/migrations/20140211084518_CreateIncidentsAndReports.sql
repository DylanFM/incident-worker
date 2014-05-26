-- +goose Up
CREATE TABLE incidents (
  uuid uuid DEFAULT uuid_generate_v4() PRIMARY KEY,
  rfs_id integer NOT NULL UNIQUE,
  current boolean DEFAULT true NOT NULL, -- Default true because they're created when they appear in the list of current incidents
  current_from tstzrange NOT NULL,
  created_at timestamp with time zone DEFAULT timezone('UTC', NOW()) NOT NULL,
  updated_at timestamp with time zone DEFAULT timezone('UTC', NOW()) NOT NULL
);
CREATE TABLE reports (
  uuid uuid DEFAULT uuid_generate_v4() PRIMARY KEY,
  incident_uuid uuid REFERENCES incidents (uuid) ON DELETE RESTRICT,
  hash text NOT NULL,
  guid text NOT NULL,
  title text NOT NULL,
  link text,
  category text,
  pubdate timestamp with time zone NOT NULL,
  description text,
  updated timestamp with time zone NOT NULL,
  alert_level text,
  location text,
  council_area text,
  status text,
  fire_type text,
  fire boolean DEFAULT true NOT NULL,
  size text,
  responsible_agency text,
  extra text,
  point geography(Point,4326),
  geometry geometry,
  created_at timestamp with time zone DEFAULT timezone('UTC', NOW()) NOT NULL,
  updated_at timestamp with time zone DEFAULT timezone('UTC', NOW()) NOT NULL
);

CREATE INDEX current_incidents_index ON incidents (current);
CREATE INDEX current_from_incidents_index ON incidents USING GiST(current_from);
CREATE INDEX reports_incident_uuid_index ON reports (incident_uuid);
CREATE INDEX report_hash_index ON reports (hash);
CREATE INDEX report_created_at_index ON reports (created_at);
CREATE INDEX report_updated_index ON reports (updated);
CREATE INDEX report_fire_index ON reports (fire);
CREATE INDEX report_geometry_index ON reports USING gist (geometry);
CREATE INDEX report_point_index ON reports USING gist (point);

-- +goose Down
DROP INDEX current_incidents_index;
DROP INDEX reports_incident_uuid_index;
DROP INDEX report_hash_index;
DROP INDEX report_created_at_index;
DROP INDEX report_updated_index;
DROP INDEX report_fire_index;
DROP INDEX report_geometry_index;
DROP INDEX report_point_index;

DROP TABLE reports;
DROP TABLE incidents;

