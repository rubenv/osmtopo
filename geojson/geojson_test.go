package geojson

import (
	"encoding/json"
	"testing"

	"github.com/cheekybits/is"
)

func TestRoundTripPolygon(t *testing.T) {
	is := is.New(t)

	in := `{"type":"Feature","geometry":{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1],[0,1],[0,0]]]},"properties":null}`

	f := &Feature{}
	err := json.Unmarshal([]byte(in), f)
	is.NoErr(err)

	geom, err := f.ToGeos()
	is.NoErr(err)
	is.NotNil(geom)

	f2, err := FromGeos(geom)
	is.NoErr(err)
	is.NotNil(f2)

	j2, err := json.Marshal(f2)
	is.NoErr(err)
	is.NotNil(j2)
	is.Equal(in, j2)
}

func TestRoundTripPolygonObj(t *testing.T) {
	is := is.New(t)

	f := &Feature{
		Type: "Feature",
		Geometry: &Geometry{
			Type: "Polygon",
			Coordinates: [][]Coordinate{
				[]Coordinate{
					Coordinate{0, 0},
					Coordinate{1, 0},
					Coordinate{1, 1},
					Coordinate{0, 1},
					Coordinate{0, 0},
				},
			},
		},
	}

	in, err := json.Marshal(f)
	is.NoErr(err)
	is.NotNil(in)

	geom, err := f.ToGeos()
	is.NoErr(err)
	is.NotNil(geom)

	f2, err := FromGeos(geom)
	is.NoErr(err)
	is.NotNil(f2)

	j2, err := json.Marshal(f2)
	is.NoErr(err)
	is.NotNil(j2)
	is.Equal(in, j2)
}
