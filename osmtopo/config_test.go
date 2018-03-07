package osmtopo

import (
	"strings"
	"testing"

	"github.com/cheekybits/is"
)

func TestParseConfig(t *testing.T) {
	is := is.New(t)

	in := `
sources:
    luxembourg:
        seed: http://download.geofabrik.de/europe/luxembourg-latest.osm.pbf
        update: http://download.geofabrik.de/europe/luxembourg-updates/

layers:
    - id: districts
      name: Districts
      admin_levels: [4]
    - id: cities
      name: Cities
      admin_levels: [8]
`

	cfg, err := ParseConfig(strings.NewReader(in))
	is.NoErr(err)
	is.NotNil(cfg)
	is.Equal(cfg.Water, DefaultWaterPolygons)
	is.Equal(len(cfg.Sources), 1)

	s, ok := cfg.Sources["luxembourg"]
	is.True(ok)
	is.Equal(s.Seed, "http://download.geofabrik.de/europe/luxembourg-latest.osm.pbf")
	is.Equal(s.Update, "http://download.geofabrik.de/europe/luxembourg-updates/")

	is.Equal(len(cfg.Layers), 2)
	l := cfg.Layers[0]
	is.Equal(l.ID, "districts")
	is.Equal(l.Name, "Districts")
}
