package cmd

import "fmt"

type CmdImport struct {
	global *GlobalOptions
}

func init() {
	_, err := parser.AddCommand("import",
		"import PBF files",
		"Imports full PBF dumps",
		&CmdImport{global: &globalOpts})
	if err != nil {
		panic(err)
	}
}

func (cmd CmdImport) Usage() string {
	return "data.osm.pbf"
}

func (cmd CmdImport) Execute(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("PBF file not specified, Usage: %s", cmd.Usage())
	}

	store, err := cmd.global.OpenStore()
	if err != nil {
		return err
	}

	err = store.Import(args[0])
	if err != nil {
		return fmt.Errorf("Failed to import: %s\n", err.Error())
	}

	return nil
}
