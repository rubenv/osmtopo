package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/cheggaaa/pb"
)

type CmdWater struct {
	global *GlobalOptions
}

func init() {
	_, err := parser.AddCommand("water",
		"Manage water polygon",
		"Download, import, simplify and export water polygons",
		&CmdWater{global: &globalOpts})
	if err != nil {
		panic(err)
	}
}

func (cmd CmdWater) Usage() string {
	return "[download|import|export] filename"
}

func (cmd CmdWater) Execute(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("Options missing, Usage: %s", cmd.Usage())
	}

	filename := args[1]

	store, err := cmd.global.OpenStore()
	if err != nil {
		return err
	}

	water := store.Water()

	switch args[0] {
	case "download":
		out, err := os.Create(filename)
		if err != nil {
			return err
		}
		defer out.Close()

		resp, err := http.Get("http://data.openstreetmapdata.com/water-polygons-split-4326.zip")
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
		return water.Import(filename)
	case "export":
		return water.Export(filename)
	}

	return nil
}
