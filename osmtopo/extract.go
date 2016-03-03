package osmtopo

import (
	"fmt"

	"github.com/kr/pretty"
)

type ExtractConfig struct {
	Languages []string `yaml:"languages"`

	Countries map[string]*CountryConfig `yaml:"countries"`
}

type CountryConfig struct {
	ID     string                  `yaml:"id"`
	Layers map[string]*LayerConfig `yaml:"layers"`
}

type LayerConfig struct {
	AdminLevel int `yaml:"admin_level"`
}

type Extractor struct {
	store   *Store
	config  *ExtractConfig
	outPath string
}

func (e *Extractor) Run() error {
	fmt.Printf("%# v\n", pretty.Formatter(e.config))
	return nil
}
