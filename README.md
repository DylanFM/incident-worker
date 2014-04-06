# Incident worker

_Part of a collection of tools used to provide an API to NSW bushfire data_: [Data collector](https://github.com/dylanfm/major-incidents-data), [Importer (this repo)](https://github.com/DylanFM/incident-worker) and [GeoJSON API](https://github.com/DylanFM/bushfires)

`incidentworker` imports GeoRSS from the NSW RFS [major incidents GeoRSS feed](http://www.rfs.nsw.gov.au/feeds/majorIncidents.xml) into a Postgres database. The command-line interface supports importing from a local file or over the internets.

For me, this tool addresses 2 needs as I build a [GeoJSON API](https://github.com/DylanFM/bushfires) for current and past bushfire data:

1. Since 30 Oct 2013 I've been collecting hourly snapshots of the GeoRSS feed. `incidentworker` will import these XML files. Iterating through the files in chronological order and importing allows for seeding of about 4,000 incidents and 19,000 reports from 2013-14 summer. I'll perform this large import once upon deployment, and push that database up to production.
2. Once launched I'll be collecting data using this worker directly with the RFS GeoRSS feed, scheduled using cron or Heroku's scheduler. I'm not certain how often, but I expect it'll check every 5 minutes or so.

## Usage

Use the command line interface to import data from an XML file locally or online.

`incidentworker` imports the data into a PostgreSQL database and makes use of the `postgis` and `uuid-ossp` extensions. The database can be created using [Goose](https://bitbucket.org/liamstask/goose/), which uses the SQL migrations located in [db/migrations](https://github.com/DylanFM/incident-worker/tree/master/db).

Configure the database for Goose by copying the file [dbconf.yml.example](https://github.com/DylanFM/incident-worker/blob/master/db/dbconf.yml.example) to `dbconf.yml`. If you prefer not to use the `DATABASE_URL` environment variable, edit `dbconf.yml` with your database connection details. Ensure the database has been created, then run `goose up` to get it in order.

### Import a local file

```
$ incidentworker /path/to/georss.xml
```

### Import a remote file

```
$ incidentworker http://www.rfs.nsw.gov.au/feeds/majorIncidents.xml
```

### Import a collection of files

Previously `incidentworker` supported importing from a directory, but since I've removed that and use a script such as this:

```bash
#!/bin/bash

for file in /path/to/major-incidents-data/*.xml; do ./incidentworker $file; done
```

This is how I've been importing the data collected by [this](https://github.com/dylanfm/major-incidents-data). To import 5 months of hourly GeoRSS feeds currently takes about 5 minutes.