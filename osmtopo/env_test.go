package osmtopo

import (
	"encoding/json"
	"log"
	"testing"

	"github.com/cheekybits/is"
)

func TestEnv(t *testing.T) {
	is := is.New(t)

	config, err := ReadConfig("../config-europe.yaml")
	is.NoErr(err)

	env, err := prepareEnv(config, "../tmp/europe-mapping.yaml", "../tmp/topo-europe", "../tmp/out-europe")
	is.NoErr(err)
	is.NotNil(env)

	rel, err := env.GetRelation(6482207)
	is.NoErr(err)
	is.NotNil(rel)

	g, err := ToGeometry(rel, env)
	is.NoErr(err)

	geom, err := GeometryFromGeos(g)
	is.NoErr(err)

	data, err := json.Marshal(geom)
	is.NoErr(err)
	log.Println(string(data))
}
