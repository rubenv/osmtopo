package osmtopo

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/cheekybits/is"
)

func TestExtract(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	is := is.New(t)

	folder, err := ioutil.TempDir("", "test")
	is.NoErr(err)
	defer os.RemoveAll(folder)
	//log.Println(folder)

	store, err := NewStore(path.Join(folder, "data"))
	is.NoErr(err)
	is.NotNil(store)

	water := store.Water()
	is.NotNil(water)

	err = water.Import("testconfigs/geodata/water-cropped.zip")
	is.NoErr(err)

	err = store.Import("testconfigs/geodata/isle-of-man-latest.osm.pbf")
	is.NoErr(err)

	outFolder := path.Join(folder, "out")
	err = store.Extract("testconfigs/man/config.yaml", outFolder)
	is.NoErr(err)

	isFile(is, path.Join(outFolder, "0", "toplevel.geojson"))
	isFile(is, path.Join(outFolder, "0", "toplevel.topojson"))
	isFile(is, path.Join(outFolder, "1", "isle-of-man.geojson"))
	isFile(is, path.Join(outFolder, "1", "isle-of-man.topojson"))
	isFile(is, path.Join(outFolder, "2", "isle-of-man-middle.geojson"))
	isFile(is, path.Join(outFolder, "2", "isle-of-man-middle.topojson"))
}

func isFile(is is.I, path string) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		is.Fail("File does not exist: ", path)
	}
	is.NoErr(err)
}
