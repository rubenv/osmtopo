package osmtopo

import (
	"io/ioutil"

	yaml "gopkg.in/yaml.v1"
)

type ExtractConfig struct {
	Languages []string          `yaml:"languages"`
	Layers    map[string]*Layer `yaml:"layers"`
}

type Layer struct {
	Load  string          `yaml:"load"`
	Items []*ItemSelector `yaml:"items"`
}

type ItemSelector struct {
	Name string      `yaml:"name"`
	ID   int64       `yaml:"id'`
	Clip [][]float64 `yaml:"clip"`
}

func LoadConfig(configPath string) (*ExtractConfig, error) {
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	config := &ExtractConfig{}
	err = yaml.Unmarshal(data, config)
	if err != nil {
		return nil, err
	}

	return config, nil
}
