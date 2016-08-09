# OpenStreetMap topology extraction tools

Tools to work with OpenStreetMap topology data, ideal if you want to extract
useful shapes for use in maps.

Work in progress!

[![Build Status](https://travis-ci.org/rubenv/osmtopo.svg?branch=master)](https://travis-ci.org/rubenv/osmtopo)

## Quick start

Get a suitable water polygon first:

```
osmtopo -d /path/to/store water download /tmp/water.zip
```

Import it:

```
osmtopo -d /path/to/store water import /tmp/water.zip
```
