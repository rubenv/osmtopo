package osmtopo

import (
	"encoding/json"
	"testing"

	"github.com/cheekybits/is"
	"github.com/rubenv/osmtopo/geojson"
)

func TestRoundTripPolygon(t *testing.T) {
	is := is.New(t)

	in := `{"type":"Feature","geometry":{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1],[0,1],[0,0]]]},"properties":null}`

	f := &geojson.Feature{}
	err := json.Unmarshal([]byte(in), f)
	is.NoErr(err)

	geom, err := FeatureToGeos(f)
	is.NoErr(err)
	is.NotNil(geom)

	f2, err := FeatureFromGeos(geom)
	is.NoErr(err)
	is.NotNil(f2)

	j2, err := json.Marshal(f2)
	is.NoErr(err)
	is.NotNil(j2)
	is.Equal(in, j2)
}

func TestRoundTripPolygonObj(t *testing.T) {
	is := is.New(t)

	f := &geojson.Feature{
		Type: "Feature",
		Geometry: &geojson.Geometry{
			Type: "Polygon",
			Coordinates: [][]geojson.Coordinate{
				{
					{0, 0},
					{1, 0},
					{1, 1},
					{0, 1},
					{0, 0},
				},
			},
		},
	}

	in, err := json.Marshal(f)
	is.NoErr(err)
	is.NotNil(in)

	geom, err := FeatureToGeos(f)
	is.NoErr(err)
	is.NotNil(geom)

	f2, err := FeatureFromGeos(geom)
	is.NoErr(err)
	is.NotNil(f2)

	j2, err := json.Marshal(f2)
	is.NoErr(err)
	is.NotNil(j2)
	is.Equal(in, j2)
}
