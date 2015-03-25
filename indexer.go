package osmtopo

import (
	"fmt"
	"strings"

	"github.com/gogo/protobuf/proto"
	"github.com/jmhodges/levigo"
)

type Indexer struct {
	store *Store
}

func (i *Indexer) newRelation(rel *Relation, wb *levigo.WriteBatch) {
	if v, ok := rel.GetTag("admin_level"); ok {
		i.indexTag(rel.GetId(), "admin_level", v, wb)
	}

	if v, ok := rel.GetTag("name"); ok {
		i.indexTag(rel.GetId(), "name", strings.ToLower(v), wb)
	}
}

func (i *Indexer) removeRelation(rel *Relation, wb *levigo.WriteBatch) {
	if v, ok := rel.GetTag("admin_level"); ok {
		i.deindexTag(rel.GetId(), "admin_level", v, wb)
	}

	if v, ok := rel.GetTag("name"); ok {
		i.deindexTag(rel.GetId(), "name", strings.ToLower(v), wb)
	}
}

func (i *Indexer) indexTag(id int64, tag, value string, wb *levigo.WriteBatch) {
	wb.Put([]byte(fmt.Sprintf("tags/%s/%s/%d", tag, value, id)), []byte("1"))
}

func (i *Indexer) deindexTag(id int64, tag, value string, wb *levigo.WriteBatch) {
	wb.Delete([]byte(fmt.Sprintf("tags/%s/%s/%d", tag, value, id)))
}

func (i *Indexer) reindex() error {
	wb := levigo.NewWriteBatch()
	defer wb.Close()

	ro := levigo.NewReadOptions()
	defer ro.Close()
	ro.SetFillCache(false)

	it := i.store.db.NewIterator(ro)
	defer it.Close()

	for it.Seek([]byte("relation")); it.Valid(); it.Next() {
		if !strings.HasPrefix(string(it.Key()), "relation") {
			break
		}

		rel := &Relation{}
		err := proto.Unmarshal(it.Value(), rel)
		if err != nil {
			return err
		}

		i.newRelation(rel, wb)
	}

	if err := it.GetError(); err != nil {
		return err
	}

	return i.store.db.Write(i.store.wo, wb)
}
