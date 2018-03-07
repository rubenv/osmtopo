package cmd

import "fmt"

type CmdExtract struct {
	global *GlobalOptions
}

func init() {
	_, err := parser.AddCommand("extract",
		"Extract topologies",
		"Extract topologies using a given configuration",
		&CmdExtract{global: &globalOpts})
	if err != nil {
		panic(err)
	}
}

func (cmd CmdExtract) Usage() string {
	return "config.yaml outputpath"
}

func (cmd CmdExtract) Execute(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("Config file or output path not specified, Usage: %s", cmd.Usage())
	}

	store, err := cmd.global.OpenStore()
	if err != nil {
		return err
	}

	err = store.Extract(args[0], args[1])
	if err != nil {
		return fmt.Errorf("Failed to extract: %s\n", err.Error())
	}

	return nil
}
