# Incident worker

_Part of a collection of tools used to provide an API to NSW bushfire data_: [Data collector](https://github.com/dylanfm/major-incidents-data), [Importer (this repo)](https://github.com/DylanFM/incident-worker) and [GeoJSON API](https://github.com/DylanFM/bushfires)

`incidentworker` imports data from the NSW Rural Fire Service's [major incidents GeoRSS feed](http://www.rfs.nsw.gov.au/feeds/majorIncidents.xml) into a database. The GeoRSS feed above contains a collection of current incidents. An incident is a fire (or something similar). Current incidents are those that have not been resolved yet.

## Why?

For me, this tool addresses 2 needs as I build a [GeoJSON API](https://github.com/DylanFM/bushfires) for current and past bushfire data:

1. Since 30 Oct 2013 I've been collecting hourly snapshots of the GeoRSS feed. `incidentworker` will import these XML files. Iterating through the files in chronological order and importing allows for seeding of about 4,000 incidents and 19,000 reports from 2013-14 summer. I'll perform this large import 1 time when I deploy, and push that database up to production.
2. Once launched I'll be collecting data using this worker directly with the RFS GeoRSS feed, scheduled using cron or Heroku's scheduler. I'm not certain how often, but I expect it'll check every 5 minutes or so.

## What's going on

In the development of `incidentworker` I've used the noun `Incident` and introduced `Report`. An incident has many reports. From this point of view, the GeoRSS feed actually contains a collection of reports, and each report relates to an incident. To be more accurate, the feed contains the most recent report for all incidents that haven't been resolved yet.

When `incidentworker` performs an import it does roughly the following for each entry (or `Report`) in the feed:

1. Have we seen the `Incident` this `Report` refers to before?
2. If no, insert the `Incident` into the database. It will be marked as `current` upon insertion.
3. If yes, ensure the existing `Incident` is marked as `current`.
4. If we haven't seen this `Report` before, insert it into the database too.
5. Ensure that the only incidents marked as `current` in the database are the ones from this update.

## Usage

Use the command line interface to import data from a local or remote XML file.

`incidentworker` imports the data into a PostgreSQL database and makes use of the `postgis` and `uuid-ossp` extensions. The database is managed in this project using [Goose](https://bitbucket.org/liamstask/goose/).

Configure the database for Goose by copying the file [dbconf.yml.example](https://github.com/DylanFM/incident-worker/blob/master/db/dbconf.yml.example) to `dbconf.yml`. If you prefer not to use the `DATABASE_URL` environment variable, edit `dbconf.yml` with your database connection details. Ensure the database has been created, then run `goose up` to run the migrations in [db/migrations](https://github.com/DylanFM/incident-worker/tree/master/db).

### Import a local file

```
$ incidentworker /path/to/georss.xml
```

### Import a remote file

```
$ incidentworker http://www.rfs.nsw.gov.au/feeds/majorIncidents.xml
```

### Import a collection of files

I use the following to import the data I've [collected](https://github.com/dylanfm/major-incidents-data). To import 5 months of hourly GeoRSS feeds currently takes about 5 minutes.

```
for file in /path/to/major-incidents-data/*.xml; do ./incidentworker $file; done
```



### Output

`incidentworker` generates the following output:

```
Importing from http://www.rfs.nsw.gov.au/feeds/majorIncidents.xml
1 new incidents, 3919 total
2 new reports, 18143 total
7 current incidents, -1 change
```

* The 1st line shows the number of new incidents encountered in this import, followed by the total number of incidents in the database.
* The 2nd line shows the number of new reports and the total number of reports in the database.
* Incidents can be current or not. The final line shows how many current incidents there are, followed by an indication of how this has changed since the previous update. e.g. in this case, there is now 1 less current incident.