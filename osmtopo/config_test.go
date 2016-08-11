package osmtopo

import (
	"testing"

	"github.com/cheekybits/is"
)

func TestConfig(t *testing.T) {
	is := is.New(t)

	c, err := ParseConfig("testconfigs/config.yaml")
	is.NoErr(err)
	is.NotNil(c)

	is.Equal(len(c.Languages), 3)
	is.Equal(c.Layer.Name, "World")
	is.Equal(c.Layer.Children[0].Name, "Europe")
	is.Equal(c.Layer.Children[0].Children[0].Name, "Belgium")
	is.Equal(c.Layer.Children[0].Children[0].ID, 52411)
}
