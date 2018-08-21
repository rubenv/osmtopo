package osmtopo

import (
	"encoding/json"
	"fmt"
	"os"
	"path"

	"github.com/rubenv/osmtopo/osmtopo/model"
)

func (e *Env) export() error {
	err := os.MkdirAll(e.outputPath, 0755)
	if err != nil {
		return err
	}

	for _, layer := range e.config.Layers {
		ids := e.topoData.Get(layer.ID)
		contains := make(map[int64]bool)
		for _, id := range ids {
			contains[id] = true
		}

		pipe := NewGeometryPipeline(e).
			Filter(func(rel *model.Relation) bool {
				return contains[rel.Id]
			}).
			Simplify(layer.Simplify).
			ClipWater().
			Quantize(1e6)

		topo, err := pipe.Run()
		if err != nil {
			return err
		}

		fp, err := os.Create(path.Join(e.outputPath, fmt.Sprintf("%s.topojson", layer.ID)))
		if err != nil {
			return err
		}

		err = json.NewEncoder(fp).Encode(topo)
		if err != nil {
			return err
		}

		fp.Close()
	}
	return nil
}
