package osmtopo

import (
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/northbright/ctx/ctxdownload"
	"github.com/omniscale/imposm3/parser/diff"
	"github.com/rubenv/osmtopo/osmtopo/model"
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
	} else {
		err = e.updateDeltas(name, source, tmp)
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *Env) importPBF(name string, source PBFSource, folder string) error {
	if source.Seed == "" {
		return fmt.Errorf("Missing seed URL for source %s", name)
	}
	e.log(fmt.Sprintf("source/%s", name), "Importing PBF")

	filename := fmt.Sprintf("%s.pbf", name)
	err := e.downloadPBF(name, folder, filename, source.Seed)
	if err != nil {
		return err
	}

	i := newImporter(e, name, path.Join(folder, filename))
	seq, err := i.Run()
	if err != nil {
		return err
	}

	e.log(fmt.Sprintf("source/%s", name), "Done")
	return e.setInt(fmt.Sprintf("seq/%s", name), seq)
}

func (e *Env) downloadPBF(name, folder, filename, url string) error {
	e.log(fmt.Sprintf("source/%s", name), "Downloading %s", url)
	buf := make([]byte, 2*1024*1024)
	_, err := ctxdownload.Download(e.ctx, url, folder, filename, buf, 24*3600)
	return err
}

func (e *Env) updateDeltas(name string, source PBFSource, folder string) error {
	if source.Seed == "" {
		return fmt.Errorf("Missing update URL for source %s", name)
	}
	key := fmt.Sprintf("seq/%s", name)
	seq, err := e.getInt(key)
	if err != nil {
		return err
	}

	current, err := fetchLatestSequence(source.Update)
	if err != nil {
		return err
	}
	if seq == current {
		return nil
	}

	e.log(fmt.Sprintf("source/%s", name), "Replicating from %d -> %d", seq, current)
	for seq < current {
		err = e.applyDelta(name, source, folder, seq)
		if err != nil {
			return err
		}
		seq++
	}

	return e.setInt(key, seq)
}

func (e *Env) applyDelta(name string, source PBFSource, folder string, seq int64) error {
	e.log(fmt.Sprintf("source/%s", name), "Replicating change %d", seq+1)
	filename, err := fetchChangeset(e.ctx, source.Update, seq, folder)
	if err != nil {
		return err
	}

	fp, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer fp.Close()

	reader, err := gzip.NewReader(fp)
	if err != nil {
		return err
	}
	defer reader.Close()

	parser := diff.NewParser(reader)
	newNodes := make([]model.Node, 0)
	newWays := make([]model.Way, 0)
	newRelations := make([]model.Relation, 0)
	for e.ctx.Err() == nil {
		elem, err := parser.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		switch {
		case elem.Del:
			if elem.Node != nil {
				n := NodeFromEl(*elem.Node)
				err = e.removeNode(n)
				if err != nil {
					return err
				}
			}
			if elem.Way != nil {
				w := WayFromEl(*elem.Way)
				err = e.removeWay(w)
				if err != nil {
					return err
				}
			}
			if elem.Rel != nil {
				r := RelationFromEl(*elem.Rel)
				err = e.removeRelation(r)
				if err != nil {
					return err
				}
			}
		case elem.Add:
			fallthrough
		case elem.Mod:
			if elem.Node != nil {
				n := NodeFromEl(*elem.Node)
				newNodes = append(newNodes, n)
			}
			if elem.Way != nil {
				w := WayFromEl(*elem.Way)
				newWays = append(newWays, w)
			}
			if elem.Rel != nil {
				r := RelationFromEl(*elem.Rel)
				if AcceptRelation(r, e.config.Blacklist) {
					newRelations = append(newRelations, r)
				}
			}
		}
	}

	// TODO: be smarter about which ways and nodes we accept

	if len(newNodes) > 0 {
		err = e.addNewNodes(newNodes)
		if err != nil {
			return err
		}
	}
	if len(newWays) > 0 {
		err = e.addNewWays(newWays)
		if err != nil {
			return err
		}
	}
	if len(newRelations) > 0 {
		err = e.addNewRelations(newRelations)
		if err != nil {
			return err
		}
	}

	return e.ctx.Err()
}
