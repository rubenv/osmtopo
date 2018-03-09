package osmtopo

import (
	"testing"

	"github.com/cheekybits/is"
)

func TestFetchLatestSequence(t *testing.T) {
	is := is.New(t)

	url := "http://planet.openstreetmap.org/replication/hour/"
	seq, err := fetchLatestSequence(url)
	is.NoErr(err)
	is.True(seq > 48098)
}
