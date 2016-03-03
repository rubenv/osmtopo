package osmtopo

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"sync"

	"github.com/kr/pretty"
	"github.com/rubenv/osmtopo/geojson"
)

type ExtractConfig struct {
	Languages []string `yaml:"languages"`

	Countries map[string]*CountryConfig `yaml:"countries"`
}

type CountryConfig struct {
	ID     int64                   `yaml:"id"`
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
	if e.config.Countries == nil {
		return errors.New("No countries defined!")
	}

	err := os.MkdirAll(e.outPath, 0755)
	if err != nil {
		return err
	}

	err = os.MkdirAll(path.Join(e.outPath, "countries"), 0755)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(len(e.config.Countries))

	for n, c := range e.config.Countries {
		name := n
		country := c

		go func() {
			defer wg.Done()
			err2 := e.ExtractCountry(name, country)
			if err2 != nil {
				err = err2
			}
		}()

	}

	wg.Wait()

	if err != nil {
		return err
	}

	return nil
}

func (e *Extractor) ExtractCountry(name string, country *CountryConfig) error {
	if country.ID == 0 {
		return fmt.Errorf("Missing ID for country: %s", name)
	}

	relation, err := e.store.GetRelation(country.ID)
	if err != nil {
		return err
	}

	feat, err := relation.ToGeometry(e.store)
	if err != nil {
		return err
	}

	out, err := geojson.FromGeos(feat)
	if err != nil {
		return err
	}

	outFile, err := os.Create(path.Join(e.outPath, "countries", fmt.Sprintf("%s.geojson", name)))
	if err != nil {
		return err
	}
	defer outFile.Close()

	err = json.NewEncoder(outFile).Encode(out)
	if err != nil {
		return err
	}

	fmt.Printf("%# v\n", pretty.Formatter(country))
	return nil
}
