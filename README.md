# Incident worker

_Part of a collection of tools used to provide an API to NSW bushfire data_: [Data collector](https://github.com/dylanfm/major-incidents-data), [Importer (this repo)](https://github.com/DylanFM/incident-worker) and [GeoJSON API](https://github.com/DylanFM/bushfires)

`incidentworker` imports data from the NSW Rural Fire Service's [major incidents GeoJSON](http://www.rfs.nsw.gov.au/feeds/majorIncidents.json) into a database. The GeoJSON above contains a collection of current incidents and behaves just like the GeoRSS feed we previously imported. An incident is a fire (or something similar). Current incidents are those that have not been resolved yet.

## What's going on

In the development of `incidentworker` I've used the noun `Incident` and introduced `Report`. An incident has many reports. From this point of view, the RFS feeds actually contain a collection of reports, and each report relates to an incident. To be more accurate, the feed contains the most recent report for all incidents that haven't been resolved yet.

When `incidentworker` performs an import it does roughly the following for each entry (or `Report`) in the feed:

1. Have we seen the `Incident` this `Report` refers to before?
2. If no, insert the `Incident` into the database. It will be marked as `current` upon insertion.
3. If yes, ensure the existing `Incident` is marked as `current`.
4. If we haven't seen this `Report` before, insert it into the database too.
5. Ensure that the only incidents marked as `current` in the database are the ones from this update.

## Usage

Use the command line interface to import data from a local or remote XML file.

`incidentworker` imports the data into a PostgreSQL database and makes use of the `postgis` and `uuid-ossp` extensions. The database is managed in this project using [Goose](https://bitbucket.org/liamstask/goose/).

Configure the database for Goose by copying the file [dbconf.yml.example](https://github.com/DylanFM/incident-worker/blob/master/db/dbconf.yml.example) to `dbconf.yml`. The database is configured by default with a `DATABASE_URL` environment variable, e.g. `postgres://user:pass@localhost/database_name?sslmode=disable`. Alternatively you can edit `dbconf.yml` with your database connection details. Ensure the database has been created, then run `goose up` to run the migrations in [db/migrations](https://github.com/DylanFM/incident-worker/tree/master/db).

### Import a local file

```
$ incidentworker /path/to/geojson.json
```

### Import a remote file

```
$ incidentworker http://www.rfs.nsw.gov.au/feeds/majorIncidents.json
```

### Import at an interval

To perform an import repeatedly at an interval, include the `--tick` option with the number of seconds between each import. This is what I'm using on Heroku to perform regular imports (refer to the Procfile).

This command will import the data every 5 minutes:

```
$ incidentworker --tick 300 http://www.rfs.nsw.gov.au/feeds/majorIncidents.json
```

### Import a collection of files

I use the following to import the data I've [collected](https://github.com/dylanfm/major-incidents-data). To import 5 months of hourly GeoRSS feeds currently takes about 5 minutes. If you wish to do this, you'll need to use an earlier version of this library as it has now switched to importing GeoJSON. The better option is just to contact me for a dump of the production database.

```
for file in /path/to/major-incidents-data/*.xml; do ./incidentworker $file; done
```
