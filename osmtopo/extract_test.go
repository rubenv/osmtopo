package osmtopo

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/cheekybits/is"
)

func TestExtract(t *testing.T) {
	/*
		if testing.Short() {
			t.Skip("skipping test in short mode.")
		}
	*/
	is := is.New(t)

	folder, err := ioutil.TempDir("", "test")
	is.NoErr(err)
	defer os.RemoveAll(folder)

	store, err := NewStore(path.Join(folder, "data"))
	is.NoErr(err)
	is.NotNil(store)

	water := store.Water()
	is.NotNil(water)

	err = water.Import("testconfigs/geodata/water-cropped.zip")
	is.NoErr(err)

	err = store.Import("testconfigs/geodata/isle-of-man-latest.osm.pbf")
	is.NoErr(err)

	err = store.Reindex()
	is.NoErr(err)

	outFolder := path.Join(folder, "out")
	err = store.Extract("testconfigs/man/config.yaml", outFolder)
	is.NoErr(err)
}
