package cmd

import "fmt"

type CmdReplicate struct {
	global *GlobalOptions
}

func init() {
	_, err := parser.AddCommand("replicate",
		"replicate planet.osm",
		"Replicates planet.osm, using daily update deltas",
		&CmdReplicate{global: &globalOpts})
	if err != nil {
		panic(err)
	}
}

func (cmd CmdReplicate) Execute(args []string) error {
	store, err := cmd.global.OpenStore()
	if err != nil {
		return err
	}

	err = store.Replicate()
	if err != nil {
		return fmt.Errorf("Failed to replicate: %s\n", err.Error())
	}
	return nil
}
