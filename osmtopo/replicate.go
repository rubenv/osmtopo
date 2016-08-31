package osmtopo

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/cheggaaa/pb"
)

const OsmServer = "http://ftp5.gwdg.de/pub/misc/openstreetmap/planet.openstreetmap.org"

func Replicate(store *Store) error {
	// Figure out if we have planet imported or not
	h, err := store.GetConfig("have_planet")
	if err != nil {
		return err
	}
	have_planet := h == "1"

	// Find replication sequence
	seq, err := store.GetConfig("seq")
	if err != nil {
		return err
	}

	currSeq := int64(0)
	if seq != "" {
		n, err := strconv.ParseInt(seq, 10, 64)
		if err != nil {
			return err
		}
		currSeq = n
	}

	if currSeq == 0 {
		n, err := fetchInitialSequence()
		if err != nil {
			return err
		}
		currSeq = n
	}

	// First import the planet, if needed
	if !have_planet {
		log.Println("Downloading planet.osm")
		filename, err := downloadPlanet()
		if err != nil {
			return err
		}
		defer os.Remove(filename)

		log.Println("Importing planet.osm")
		err = store.Import(filename)
		if err != nil {
			return err
		}

		err = store.SetConfig("have_planet", "1")
		if err != nil {
			return err
		}
	}

	latestSeq, err := fetchCurrentSequence()
	if err != nil {
		return err
	}

	if currSeq != latestSeq {
		behind := latestSeq - currSeq
		fmt.Printf("Currently %d updates behind %d, updating...\n", behind, latestSeq)

		for currSeq < latestSeq {
			newSeq := currSeq + 1
			err = applyChangeSet(store, newSeq)
			if err != nil {
				return err
			}

			err = store.SetConfig("seq", fmt.Sprintf("%d", newSeq))
			if err != nil {
				return err
			}

			currSeq = newSeq
		}
	}

	return nil
}

func fetchInitialSequence() (int64, error) {
	seq, err := fetchCurrentSequence()
	if err != nil {
		return 0, err
	}

	// Slightly offset sequence to make sure we overlap with
	// the current planet.osm file
	return seq - 7, err
}

func fetchCurrentSequence() (int64, error) {
	url := fmt.Sprintf("%s/replication/day/state.txt", OsmServer)

	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	currSeq := int64(0)
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if parts[0] == "sequenceNumber" {
			n, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				return 0, err
			}
			currSeq = n
		}
	}

	err = scanner.Err()
	if err != nil {
		return 0, err
	}

	if currSeq == 0 {
		return 0, errors.New("No sequenceNumber found")
	}
	return currSeq, nil
}

func downloadPlanet() (string, error) {
	url := fmt.Sprintf("%s/pbf/planet-latest.osm.pbf", OsmServer)
	return downloadProgress("planet", url)
}

func downloadChangeSet(seq int64) (string, error) {
	s := fmt.Sprintf("%09d", seq)
	url := fmt.Sprintf("%s/replication/day/%s/%s/%s.osc.gz", OsmServer, s[0:3], s[3:6], s[6:])
	return downloadProgress("change", url)
}

func downloadProgress(tmpName, url string) (string, error) {
	tmp, err := ioutil.TempFile("", tmpName)
	if err != nil {
		return "", err
	}
	defer tmp.Close()

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bar := pb.New(int(resp.ContentLength)).SetUnits(pb.U_BYTES)
	bar.Start()
	defer bar.Finish()

	reader := bar.NewProxyReader(resp.Body)

	_, err = io.Copy(tmp, reader)
	if err != nil {
		return "", err
	}
	return tmp.Name(), err
}

func applyChangeSet(store *Store, seq int64) error {
	fmt.Printf("Downloading %d...\n", seq)
	filename, err := downloadChangeSet(seq)
	if err != nil {
		return err
	}
	defer os.Remove(filename)

	fmt.Printf("Applying %d...\n", seq)
	return store.ApplyChange(filename)
}
