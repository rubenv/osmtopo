package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/cheggaaa/pb"
)

type CmdLand struct {
	global *GlobalOptions
}

func init() {
	_, err := parser.AddCommand("land",
		"Manage land polygon",
		"Download, import, simplify and export land polygons",
		&CmdLand{global: &globalOpts})
	if err != nil {
		panic(err)
	}
}

func (cmd CmdLand) Usage() string {
	return "[download|import|export] filename"
}

func (cmd CmdLand) Execute(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("Options missing, Usage: %s", cmd.Usage())
	}

	filename := args[1]

	store, err := cmd.global.OpenStore()
	if err != nil {
		return err
	}

	land := store.Land()

	switch args[0] {
	case "download":
		out, err := os.Create(filename)
		if err != nil {
			return err
		}
		defer out.Close()

		resp, err := http.Get("http://data.openstreetmapdata.com/land-polygons-complete-4326.zip")
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		bar := pb.New(int(resp.ContentLength)).SetUnits(pb.U_BYTES).Format("[=> ]")
		bar.Start()

		reader := bar.NewProxyReader(resp.Body)
		_, err = io.Copy(out, reader)
		return err
	case "import":
		return land.Import(filename)
	}

	return nil
}
