# OpenStreetMap topology extraction tools

Tools to work with OpenStreetMap topology data, ideal if you want to extract
useful shapes for use in maps.

Work in progress!

[![Build Status](https://travis-ci.org/rubenv/osmtopo.svg?branch=master)](https://travis-ci.org/rubenv/osmtopo)

## Installing

```
go install github.com/rubenv/osmtopo/bin/osmtopo
```

Or use the Docker image, which is available here: https://hub.docker.com/r/rubenv/osmtopo/

```
alias osmtopo="docker run -ti --rm -v $(pwd):/data docker.io/rubenv/osmtopo"
```

Be sure to put your data store in /data when using the alias above

## Quick start

Get a suitable water polygon first:

```
osmtopo -d /path/to/store water download /tmp/water.zip
```

Import it:

```
osmtopo -d /path/to/store water import /tmp/water.zip
```

Find a suitable data set:

* Either one of the PBF files at GeoFabrik: http://download.geofabrik.de/
* Or the entire world: http://planet.openstreetmap.org/pbf/planet-latest.osm.pbf

Note that big data sets take a long time to process, you've been warned!

Import it:

```
osmtopo -d /path/to/store import belgium-latest.osm.pbf netherlands-latest.osm.pbf
```

## Troubleshooting

### panic: file open: open belgium-latest.osm.pbf: too many open files

You're probably using a Mac, which has idiotically low open file limits by default.

Try the following:

```
ulimit -n 1024
```
