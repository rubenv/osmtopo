package osmtopo

import (
	"io"
	"os"
	"sort"

	yaml "gopkg.in/yaml.v2"
)

type TopologyData struct {
	Layers map[string]IDSlice `yaml:"layers" json:"layers"`
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
		Layers: make(map[string]IDSlice),
	}
}

func (t *TopologyData) Add(layer string, id int64) {
	if !t.Contains(layer, id) {
		t.Layers[layer] = append(t.Layers[layer], id)
		sort.Sort(t.Layers[layer])
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

func (t *TopologyData) Get(layer string) IDSlice {
	return t.Layers[layer]
}

func (t *TopologyData) WriteTo(filename string) error {
	fp, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer fp.Close()
	return yaml.NewEncoder(fp).Encode(t)
}

type IDSlice []int64

func (p IDSlice) Len() int           { return len(p) }
func (p IDSlice) Less(i, j int) bool { return p[i] < p[j] }
func (p IDSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
