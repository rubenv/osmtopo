package cmd

type CmdServer struct {
	global *GlobalOptions
}

func init() {
	_, err := parser.AddCommand("server",
		"Run topo configuration server",
		"Run topo configuration server\n\nAllows you to define shapes",
		&CmdServer{global: &globalOpts})
	if err != nil {
		panic(err)
	}
}

func (cmd CmdServer) Usage() string {
	return ""
}

func (cmd CmdServer) Execute(args []string) error {
	return nil
}
