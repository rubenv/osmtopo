package osmtopo

import (
	"io/ioutil"
	"path"

	yaml "gopkg.in/yaml.v1"
)

type Config struct {
	Languages []string     `yaml:"languages"`
	Simplify  []int        `yaml:"simplify"`
	Layer     *ConfigLayer `yaml:"layer"`
}

type ConfigLayer struct {
	ID   int64       `yaml:"id"`
	Name string      `yaml:"name"`
	Load string      `yaml:"load"`
	Clip [][]float64 `yaml:"clip"`

	Output string `yaml:"-"`

	Children []*ConfigLayer `yaml:"children"`
}

func (c *ConfigLayer) processLoad(folder string) error {
	if c.Load != "" {
		data, err := ioutil.ReadFile(path.Join(folder, c.Load))
		if err != nil {
			return err
		}

		err = yaml.Unmarshal(data, c)
		if err != nil {
			return err
		}
	}

	for _, child := range c.Children {
		err := child.processLoad(folder)
		if err != nil {
			return err
		}
	}

	return nil
}

func ParseConfig(configPath string) (*Config, error) {
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	config := &Config{}
	err = yaml.Unmarshal(data, config)
	if err != nil {
		return nil, err
	}

	err = config.Layer.processLoad(path.Dir(configPath))
	if err != nil {
		return nil, err
	}

	return config, nil
}
