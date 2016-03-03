package cmd

import "fmt"

type CmdApplyChange struct {
	global *GlobalOptions
}

func init() {
	_, err := parser.AddCommand("apply-change",
		"Apply changeset",
		"Apply delta changes to data store",
		&CmdApplyChange{global: &globalOpts})
	if err != nil {
		panic(err)
	}
}

func (cmd CmdApplyChange) Usage() string {
	return "data.osc.gz"
}

func (cmd CmdApplyChange) Execute(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("OSC file not specified, Usage: %s", cmd.Usage())
	}

	store, err := cmd.global.OpenStore()
	if err != nil {
		return err
	}

	err = store.ApplyChange(args[0])
	if err != nil {
		return fmt.Errorf("Failed to apply changes: %s\n", err.Error())
	}

	return nil
}
