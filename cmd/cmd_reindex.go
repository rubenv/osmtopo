package cmd

import "fmt"

type CmdReindex struct {
	global *GlobalOptions
}

func init() {
	_, err := parser.AddCommand("reindex",
		"Reindex data store",
		"Reindex the whole data store",
		&CmdReindex{global: &globalOpts})
	if err != nil {
		panic(err)
	}
}

func (cmd CmdReindex) Execute(args []string) error {
	store, err := cmd.global.OpenStore()
	if err != nil {
		return err
	}

	err = store.Reindex()
	if err != nil {
		return fmt.Errorf("Failed to reindex: %s\n", err.Error())
	}
	return nil
}
