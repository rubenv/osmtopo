package osmtopo

import (
	"io"
	"os"

	yaml "gopkg.in/yaml.v2"
)

type TopologyData struct {
	Layers map[string][]int64 `yaml:"layers" json:"layers"`
}

func ReadTopologies(filename string) (*TopologyData, error) {
	_, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return NewTopologyData(), nil
	}
	if err != nil {
		return nil, err
	}

	fp, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer fp.Close()
	return ParseTopologies(fp)
}

func ParseTopologies(in io.Reader) (*TopologyData, error) {
	c := NewTopologyData()
	err := yaml.NewDecoder(in).Decode(c)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func NewTopologyData() *TopologyData {
	return &TopologyData{
		Layers: make(map[string][]int64),
	}
}

func (t *TopologyData) Add(layer string, id int64) {
	if !t.Contains(layer, id) {
		t.Layers[layer] = append(t.Layers[layer], id)
	}
}

func (t *TopologyData) Contains(layer string, id int64) bool {
	ids, ok := t.Layers[layer]
	if !ok {
		return false
	}

	found := false
	for _, i := range ids {
		if i == id {
			found = true
			break
		}
	}
	return found
}

func (t *TopologyData) WriteTo(filename string) error {
	fp, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer fp.Close()
	return yaml.NewEncoder(fp).Encode(t)
}
