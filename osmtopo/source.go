package osmtopo

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/northbright/ctx/ctxdownload"
)

func (e *Env) updateSource(name string, source PBFSource) error {
	stamp := fmt.Sprintf("source/%s", name)
	shouldRun, err := e.shouldRun(stamp, 3600)
	if err != nil {
		return err
	}
	if !shouldRun {
		return nil
	}

	flag := fmt.Sprintf("imported/%s", name)
	imported, err := e.getFlag(flag)
	if err != nil {
		return err
	}

	tmp, err := ioutil.TempDir("", fmt.Sprintf("source-%s", name))
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	if !imported {
		err := e.importPBF(name, source, tmp)
		if err != nil {
			return err
		}

		err = e.setFlag(flag, true)
		if err != nil {
			return err
		}
	}

	e.log(fmt.Sprintf("source/%s", name), "Done")
	return nil
}

func (e *Env) importPBF(name string, source PBFSource, folder string) error {
	e.log(fmt.Sprintf("source/%s", name), "Importing PBF")

	filename := fmt.Sprintf("%s.pbf", name)
	err := e.downloadPBF(name, folder, filename, source.Seed)
	if err != nil {
		return err
	}

	i := newImporter(e, name, path.Join(folder, filename))
	return i.Run()
}

func (e *Env) downloadPBF(name, folder, filename, url string) error {
	e.log(fmt.Sprintf("source/%s", name), "Downloading %s", url)
	buf := make([]byte, 2*1024*1024)
	_, err := ctxdownload.Download(e.ctx, url, folder, filename, buf, 24*3600)
	return err
}
