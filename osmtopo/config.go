package osmtopo

import (
	"io"
	"os"
	"time"

	yaml "gopkg.in/yaml.v2"
)

const Day = 24 * time.Hour
const DefaultWaterPolygons = "http://data.openstreetmapdata.com/water-polygons-split-4326.zip"
const DefaultWaterUpdate = 4 * 7 * Day
const DefaultExportPointLimit = 10000

type Config struct {
	// Where to download water polygons
	//
	// Should not be set most of the time. When unspecified
	// DefaultWaterPolygons is used.
	Water string `yaml:"water" json:"water"`

	// Sources to load OSM data from
	Sources map[string]PBFSource `yaml:"sources" json:"sources"`

	// Output layers
	Layers []Layer `yaml:"layers" json:"layers"`

	// Blacklist features
	Blacklist []int64 `yaml:"blacklist" json:"blacklist"`

	// **** Bits below usually don't need to be set ****

	// Update interval in seconds, defaults to every 4 weeks
	UpdateWaterEvery int64 `yaml:"update_water_every" json:"update_water_every"`

	// Target number of points in generated topojson files
	ExportPointLimit int `yaml:"export_point_limit" json:"export_point_limit"`
}

type PBFSource struct {
	// URL to the .osm.pbf file
	Seed string `yaml:"seed" json:"seed"`

	// URL to the .osc.gz replication files
	Update string `yaml:"update" json:"update"`
}

type Layer struct {
	ID          string `yaml:"id" json:"id"`
	Name        string `yaml:"name" json:"name"`
	AdminLevels []int  `yaml:"admin_levels" json:"admin_levels"`
	Simplify    int    `yaml:"simplify" json:"simplify"`
}

func ReadConfig(filename string) (*Config, error) {
	fp, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer fp.Close()
	return ParseConfig(fp)
}

func ParseConfig(in io.Reader) (*Config, error) {
	c := NewConfig()
	err := yaml.NewDecoder(in).Decode(c)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func NewConfig() *Config {
	return &Config{
		Water:            DefaultWaterPolygons,
		UpdateWaterEvery: int64(DefaultWaterUpdate.Seconds()),
		ExportPointLimit: DefaultExportPointLimit,
	}
}
