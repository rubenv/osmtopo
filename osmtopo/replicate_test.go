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

func TestChangesetUrl(t *testing.T) {
	is := is.New(t)

	url := "http://download.geofabrik.de/europe/monaco-updates"
	is.Equal(changesetUrl(url, 1781), url+"/000/001/781.osc.gz")
	is.Equal(changesetUrl(url, 1), url+"/000/000/001.osc.gz")
	is.Equal(changesetUrl(url, 123456789), url+"/123/456/789.osc.gz")
}
