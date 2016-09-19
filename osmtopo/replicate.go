package osmtopo

import (
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/cheggaaa/pb"
)

//const OsmServer = "http://ftp5.gwdg.de/pub/misc/openstreetmap/planet.openstreetmap.org"
const OsmServer = "http://planet.openstreetmap.org"

func Replicate(store *Store, planet_file string) error {
	// Figure out if we have planet imported or not
	h, err := store.GetConfig("have_planet")
	if err != nil {
		return err
	}
	have_planet := h == "1"

	// First import the planet, if needed
	if !have_planet {
		if planet_file != "" {
			_, err := os.Stat(planet_file)
			if err != nil {
				return err
			}
		} else {
			log.Println("Downloading planet.osm")
			filename, err := downloadPlanet()
			if err != nil {
				return err
			}
			defer os.Remove(filename)

			planet_file = filename
		}

		log.Println("Importing planet.osm")
		err = store.Import(planet_file)
		if err != nil {
			return err
		}

		err = store.SetConfig("have_planet", "1")
		if err != nil {
			return err
		}
	}

	return nil
}

func downloadPlanet() (string, error) {
	url := fmt.Sprintf("%s/pbf/planet-latest.osm.pbf", OsmServer)
	file, hash, err := downloadProgress("planet", url)
	if err != nil {
		return "", err
	}

	hresp, err := http.Get(fmt.Sprintf("%s.md5", url))
	if err != nil {
		return "", err
	}
	defer hresp.Body.Close()

	if hresp.StatusCode != 200 {
		return "", fmt.Errorf("Unexpected status code: %d", hresp.StatusCode)
	}

	resp, err := ioutil.ReadAll(hresp.Body)
	if err != nil {
		return "", err
	}

	parts := strings.Split(strings.TrimSpace(string(resp)), "  ")
	if len(parts) != 2 {
		return "", fmt.Errorf("Unexpected checksum file: %#v", string(resp))
	}
	if parts[1] != "planet-latest.osm.pbf" {
		return "", fmt.Errorf("Unexpected filename: %s", parts[1])
	}

	if parts[0] != hash {
		return "", fmt.Errorf("MD5 verification failed: %#v != %#v", parts[0], hash)
	}

	return file, nil
}

func downloadProgress(tmpName, url string) (string, string, error) {
	tmp, err := ioutil.TempFile("", tmpName)
	if err != nil {
		return "", "", err
	}
	defer tmp.Close()

	resp, err := http.Get(url)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("Unexpected status code: %d", resp.StatusCode)
	}

	bar := pb.New(int(resp.ContentLength)).SetUnits(pb.U_BYTES)
	bar.Start()
	defer bar.Finish()

	reader := bar.NewProxyReader(resp.Body)

	h := md5.New()
	w := io.MultiWriter(tmp, h)

	_, err = io.Copy(w, reader)
	if err != nil {
		return "", "", err
	}

	return tmp.Name(), fmt.Sprintf("%02x", h.Sum(nil)), err
}
