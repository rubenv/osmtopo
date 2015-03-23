package osmtopo

import (
	"fmt"

	"github.com/jmhodges/levigo"
	"github.com/omniscale/imposm3/element"
)

type Indexer struct {
	store *Store
}

func (i *Indexer) newRelation(rel element.Relation, wb *levigo.WriteBatch) {
	if v, ok := rel.Tags["admin_level"]; ok {
		i.indexTag(rel.Id, "admin_level", v, wb)
	}
}

func (i *Indexer) indexTag(id int64, tag, value string, wb *levigo.WriteBatch) {
	wb.Put([]byte(fmt.Sprintf("tags/%s/%s/%d", tag, value, id)), []byte("1"))
}
