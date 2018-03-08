package cmd

import (
	"fmt"
	"os"

	"github.com/jessevdk/go-flags"
	"github.com/rubenv/osmtopo/osmtopo"
)

type GlobalOptions struct {
	DataStore  string `short:"d" long:"datastore" description:"Data store path" required:"true"`
	Config     string `short:"c" long:"config" description:"Config file path" required:"true"`
	Topologies string `short:"t" long:"topologies" description:"Topologies mapping path" required:"true"`

	NoUpdate bool `short:"n" long:"no-update" description:"Don't update data"`
}

var globalOpts = GlobalOptions{}
var parser = flags.NewParser(&globalOpts, flags.HelpFlag|flags.PassDoubleDash)

func Run() error {
	_, err := parser.Parse()
	if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
		parser.WriteHelp(os.Stdout)
		return nil
	}
	return err
}

func (g *GlobalOptions) NewEnv() (*osmtopo.Env, error) {
	config, err := osmtopo.ReadConfig(g.Config)
	if err != nil {
		return nil, err
	}

	if g.NoUpdate {
		config.NoUpdate = true
	}

	env, err := osmtopo.NewEnv(config, g.Topologies, g.DataStore)
	if err != nil {
		return nil, fmt.Errorf("Failed to create env: %s\n", err.Error())
	}
	return env, nil
}
