package osmtopo

import (
	"fmt"

	"github.com/jmhodges/levigo"
)

type Indexer struct {
	store *Store
}

func (i *Indexer) newRelation(rel *Relation, wb *levigo.WriteBatch) {
	if v, ok := rel.GetTag("admin_level"); ok {
		i.indexTag(rel.GetId(), "admin_level", v, wb)
	}
}

func (i *Indexer) removeRelation(rel *Relation, wb *levigo.WriteBatch) {
	if v, ok := rel.GetTag("admin_level"); ok {
		i.deindexTag(rel.GetId(), "admin_level", v, wb)
	}
}

func (i *Indexer) indexTag(id int64, tag, value string, wb *levigo.WriteBatch) {
	wb.Put([]byte(fmt.Sprintf("tags/%s/%s/%d", tag, value, id)), []byte("1"))
}

func (i *Indexer) deindexTag(id int64, tag, value string, wb *levigo.WriteBatch) {
	wb.Delete([]byte(fmt.Sprintf("tags/%s/%s/%d", tag, value, id)))
}
