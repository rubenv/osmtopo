package osmtopo

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/cheekybits/is"
)

func TestExport(t *testing.T) {
	is := is.New(t)

	folder, err := ioutil.TempDir("", "test")
	is.NoErr(err)
	defer os.RemoveAll(folder)

	config := NewConfig()
	config.Water = "fixtures/geodata/water-cropped.zip"
	config.Sources = map[string]PBFSource{
		// The Isle of Man is ideal: small file size, water, regional subdivisions
		"man": PBFSource{
			Seed: "fixtures/geodata/isle-of-man-latest.osm.pbf",
		},
	}
	config.Layers = []Layer{
		{
			ID:       "countries",
			Name:     "Countries",
			Simplify: 3,
		},
		{
			ID:       "regions",
			Name:     "Regions",
			Simplify: 5,
		},
		{
			ID:       "cities",
			Name:     "Cities",
			Simplify: 6,
		},
	}
	config.ExportPointLimit = 1000

	topologiesFile := path.Join(folder, "topo.yaml")
	storePath := path.Join(folder, "store")
	outputPath := path.Join(folder, "output")

	topologies := &TopologyData{
		Layers: map[string]IDSlice{
			"countries": {62269},
			"regions": {
				1061144, // Ayre
				1061147, // Glenfaba
				1061145, // Garff
				1028022, // Michael
				1061135, // Rushen
				1061146, // Middle
			},
			"cities": {
				1061141, 1061134, 1061131, // Ayre
				1029648, 1028021, 1061133, // Glenfaba
				1061132, 1061136, 1061140, 1061137, // Garff
				1028023, 1061143, 1061139, // Michael
				1029669, 1029647, 1032811, 1034178, 1029670, 1031319, // Rushen
				1029649, 1028066, 1028024, 1061142, 1061138, // Middle
			},
		},
	}
	err = topologies.WriteTo(topologiesFile)
	is.NoErr(err)

	env, err := NewEnv(config, topologiesFile, storePath, outputPath)
	is.NoErr(err)
	is.NotNil(env)
	defer env.Stop()

	err = env.export()
	is.NoErr(err)

	isFile(is, path.Join(outputPath, "countries/0.topojson"))
	isFile(is, path.Join(outputPath, "regions/0.topojson"))
	isFile(is, path.Join(outputPath, "cities/0.topojson"))
	isFile(is, path.Join(outputPath, "cities/1.topojson"))
}

func isFile(is is.I, path string) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		is.Fail("File does not exist: ", path)
	}
	is.NoErr(err)
}
