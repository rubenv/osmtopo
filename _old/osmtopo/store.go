package osmtopo

import (
	"fmt"

	"github.com/rubenv/osmtopo/osmtopo/model"
	"github.com/tecbot/gorocksdb"
)

type Store struct {
	path string
	db   *gorocksdb.DB

	wo *gorocksdb.WriteOptions
	ro *gorocksdb.ReadOptions
}

func (s *Store) GetGeometry(prefix string, id int64) (*model.Geometry, error) {
	n, err := s.db.Get(s.ro, []byte(fmt.Sprintf("geometry/%s/%d", prefix, id)))
	if err != nil {
		return nil, err
	}
	defer n.Free()

	if n.Size() == 0 {
		return nil, nil
	}

	rel := &model.Geometry{}
	err = rel.Unmarshal(n.Data())
	if err != nil {
		return nil, err
	}

	return rel, nil
}

func (s *Store) GetConfig(key string) (string, error) {
	n, err := s.db.Get(s.ro, []byte(fmt.Sprintf("config/%s", key)))
	if err != nil {
		return "", err
	}
	defer n.Free()

	return string(n.Data()), nil
}

func (s *Store) SetConfig(key, value string) error {
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
	wb.Put([]byte(fmt.Sprintf("config/%s", key)), []byte(value))
	return s.db.Write(s.wo, wb)
}
