package osmtopo

import (
	"fmt"
	"strings"

	"github.com/gogo/protobuf/proto"
	"github.com/rubenv/osmtopo/osmtopo/model"
	"github.com/tecbot/gorocksdb"
)

type Indexer struct {
	store *Store
}

func (i *Indexer) newRelation(rel *model.Relation, wb *gorocksdb.WriteBatch) {
	if v, ok := rel.GetTag("admin_level"); ok {
		i.indexTag(rel.Id, "admin_level", v, wb)
	}

	if v, ok := rel.GetTag("name"); ok {
		i.indexTag(rel.Id, "name", strings.ToLower(v), wb)
	}
}

func (i *Indexer) removeRelation(rel *model.Relation, wb *gorocksdb.WriteBatch) {
	if v, ok := rel.GetTag("admin_level"); ok {
		i.deindexTag(rel.Id, "admin_level", v, wb)
	}

	if v, ok := rel.GetTag("name"); ok {
		i.deindexTag(rel.Id, "name", strings.ToLower(v), wb)
	}
}

func (i *Indexer) indexTag(id int64, tag, value string, wb *gorocksdb.WriteBatch) {
	wb.Put([]byte(fmt.Sprintf("tags/%s/%s/%d", tag, value, id)), []byte("1"))
}

func (i *Indexer) deindexTag(id int64, tag, value string, wb *gorocksdb.WriteBatch) {
	wb.Delete([]byte(fmt.Sprintf("tags/%s/%s/%d", tag, value, id)))
}

func (i *Indexer) reindex() error {
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()

	ro := gorocksdb.NewDefaultReadOptions()
	ro.SetFillCache(false)

	it := i.store.db.NewIterator(ro)
	defer it.Close()

	prefix := []byte("relation")
	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		rel := &model.Relation{}
		err := proto.Unmarshal(it.Value().Data(), rel)
		if err != nil {
			return err
		}

		i.newRelation(rel, wb)
	}

	if err := it.Err(); err != nil {
		return err
	}

	return i.store.db.Write(i.store.wo, wb)
}
