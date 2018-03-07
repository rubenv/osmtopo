package cmd

import (
	"os"

	"github.com/jessevdk/go-flags"
)

type GlobalOptions struct {
	DataStore  string `short:"d" long:"datastore" description:"Data store path" required:"true"`
	Config     string `short:"c" long:"config" description:"Config file path" required:"true"`
	Topologies string `short:"t" long:"topologies" description:"Topologies mapping path" required:"true"`
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

/*
func (g *GlobalOptions) OpenStore() (*osmtopo.Store, error) {
	if g.DataStore == "" {
		return nil, errors.New("No datastore specified")
	}

	store, err := osmtopo.NewStore(g.DataStore)
	if err != nil {
		return nil, fmt.Errorf("Failed to open store: %s\n", err.Error())
	}
	return store, nil
}
*/
