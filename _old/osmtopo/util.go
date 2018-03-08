package osmtopo

import (
	"encoding/binary"
)

func nodeKey(id int64) []byte {
	buf := make([]byte, 13)
	copy(buf, "node/")
	binary.BigEndian.PutUint64(buf[5:], uint64(id))
	return buf
}

func wayKey(id int64) []byte {
	buf := make([]byte, 12)
	copy(buf, "way/")
	binary.BigEndian.PutUint64(buf[4:], uint64(id))
	return buf
}

func relationKey(id int64) []byte {
	buf := make([]byte, 17)
	copy(buf, "relation/")
	binary.BigEndian.PutUint64(buf[9:], uint64(id))
	return buf
}
