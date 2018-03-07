package osmtopo

import (
	"io"
	"os"

	yaml "gopkg.in/yaml.v2"
)

const DefaultWaterPolygons = "http://data.openstreetmapdata.com/water-polygons-split-4326.zip"

type Config struct {
	// Where to download water polygons
	//
	// Should not be set most of the time. When unspecified
	// DefaultWaterPolygons is used.
	Water string `yaml:"water"`

	// Sources to load OSM data from
	Sources map[string]PBFSource `yaml:"sources"`

	// Output layers
	Layers []Layer `yaml:"layers"`
}

type PBFSource struct {
	// URL to the .osm.pbf file
	Seed string `yaml:"seed"`

	// URL to the .osc.gz replication files
	Update string `yaml:"update"`
}

type Layer struct {
	ID   string `yaml:"id"`
	Name string `yaml:"name"`
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
		Water: DefaultWaterPolygons,
	}
}
