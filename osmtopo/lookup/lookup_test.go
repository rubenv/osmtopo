package lookup

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/cheekybits/is"
	"github.com/golang/geo/s2"
	geojson "github.com/paulmach/go.geojson"
	"github.com/rubenv/topojson"
)

// Has a duplicated point, which causes all sorts of trouble if we don't filter them out
const hoornGeoJson = `{"type":"Polygon","coordinates":[[[5.0745865000000006,52.58366650000001],[5.1353057,52.603937200000004],[5.1353057,52.603937200000004],[5.1252198,52.6151739],[5.0948625000000005,52.6323037],[5.1014438,52.635095500000006],[5.1059641000000004,52.636073700000004],[5.104501900000001,52.638561900000006],[5.1019294,52.6471808],[5.1170784000000005,52.6499858],[5.1164058,52.650657800000005],[5.1194141,52.651163200000006],[5.1161157,52.6543116],[5.114759100000001,52.6537829],[5.1113401000000005,52.6572223],[5.1075911000000005,52.65972],[5.1076348000000005,52.662157500000006],[5.104862000000001,52.664295800000005],[5.104126900000001,52.6638852],[5.1008888,52.665820000000004],[5.0962662000000005,52.669722900000004],[5.0988161000000005,52.671201100000005],[5.0960503,52.674965400000005],[5.091752400000001,52.678923700000006],[5.091752400000001,52.678923700000006],[5.0873071,52.6825929],[5.086730500000001,52.684365500000006],[5.083002400000001,52.6832008],[5.0826067,52.68400320000001],[5.0622919,52.6767076],[5.062139500000001,52.675251100000004],[5.0634555,52.673133400000005],[5.0471709,52.67191570000001],[5.0400358,52.6711941],[5.0355401,52.670453],[5.035720700000001,52.6681978],[5.0347627,52.666556400000005],[5.0347627,52.666556400000005],[5.039043,52.6558636],[5.037674900000001,52.650456600000005],[5.0381434,52.648961500000006],[5.0368315,52.6473353],[5.0321597,52.6441767],[5.0225307,52.640412600000005],[5.0174895,52.637737900000005],[5.0148747,52.634995700000005],[5.0135367,52.630431400000006],[5.0204939,52.6300078],[5.027031,52.61260600000001],[5.027031,52.61260600000001],[5.0604282000000005,52.5789361],[5.0604282000000005,52.5789361],[5.0745865000000006,52.58366650000001]]]}`

func TestLookup(t *testing.T) {
	is := is.New(t)

	geom := &geojson.Geometry{}
	err := json.Unmarshal([]byte(hoornGeoJson), geom)
	is.NoErr(err)

	l := New()
	err = l.IndexGeometry("test", 291667, geom)
	is.NoErr(err)

	err = l.Build()
	is.NoErr(err)

	matches, err := l.Query(51.080501556396484, 4.464809894561768, "test")
	is.NoErr(err)
	is.Equal(len(matches), 0)
}

func TestLoops(t *testing.T) {
	is := is.New(t)

	geom := &geojson.Geometry{}
	err := json.Unmarshal([]byte(hoornGeoJson), geom)
	is.NoErr(err)

	lat := 51.080501556396484
	lng := 4.464809894561768

	latlon := s2.LatLngFromDegrees(lat, lng)
	point := s2.PointFromLatLng(latlon)

	outer := makeLoop(geom.Polygon[0])
	is.NoErr(outer.Validate())
	is.False(outer.ContainsPoint(point))
}

func TestLookupCities(t *testing.T) {
	is := is.New(t)

	fp, err := os.Open("fixtures/cities.topojson")
	is.NoErr(err)
	defer fp.Close()

	topo := &topojson.Topology{}
	err = json.NewDecoder(fp).Decode(topo)
	is.NoErr(err)

	l := New()
	err = l.IndexFeatures("cities", topo.ToGeoJSON())
	is.NoErr(err)

	err = l.Build()
	is.NoErr(err)

	ids, err := l.Query(54.1504053, -4.4776897, "cities")
	is.NoErr(err)

	found := false
	for _, id := range ids {
		if id == 1061138 {
			found = true
		}
	}
	is.True(found)
}
