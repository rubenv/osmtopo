package cmd

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type CmdServer struct {
	global *GlobalOptions

	Listen string `short:"l" long:"listen" description:"Listen on this address" default:":8888"`
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
	env, err := cmd.global.NewEnv()
	if err != nil {
		return err
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM)
	signal.Notify(stop, syscall.SIGINT)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-stop
		env.Stop()
	}()

	err = env.StartServer(cmd.Listen)
	if err != nil {
		return err
	}

	wg.Wait()
	return nil
}
