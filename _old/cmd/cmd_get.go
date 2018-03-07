package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/kr/pretty"
	"github.com/rubenv/osmtopo/osmtopo"
)

type CmdGet struct {
	global *GlobalOptions
}

func init() {
	_, err := parser.AddCommand("get",
		"Get items",
		"Get items from datastore",
		&CmdGet{global: &globalOpts})
	if err != nil {
		panic(err)
	}
}

func (cmd CmdGet) Usage() string {
	return "[node|way|relation|geometry] id"
}

func (cmd CmdGet) Execute(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("Options missing, Usage: %s", cmd.Usage())
	}

	store, err := cmd.global.OpenStore()
	if err != nil {
		return err
	}

	id, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return err
	}

	switch args[0] {
	case "node":
		node, err := store.GetNode(id)
		if err != nil {
			return fmt.Errorf("Failed to get node: %s\n", err.Error())
		}

		fmt.Printf("%# v\n", pretty.Formatter(node))
	case "way":
		way, err := store.GetWay(id)
		if err != nil {
			return fmt.Errorf("Failed to get way: %s\n", err.Error())
		}

		fmt.Printf("%# v\n", pretty.Formatter(way))
	case "relation":
		relation, err := store.GetRelation(id)
		if err != nil {
			return fmt.Errorf("Failed to get relation: %s\n", err.Error())
		}

		fmt.Printf("%# v\n", pretty.Formatter(relation))
	case "geometry":
		relation, err := store.GetRelation(id)
		if err != nil {
			return fmt.Errorf("Failed to get relation: %s\n", err.Error())
		}

		geom, err := osmtopo.ToGeometry(relation, store)
		if err != nil {
			return err
		}

		out, err := osmtopo.GeometryFromGeos(geom)
		if err != nil {
			return err
		}

		b, err := json.Marshal(out)
		if err != nil {
			return err
		}
		os.Stdout.Write(b)
		os.Stdout.WriteString("\n")
	default:
		return fmt.Errorf("Unknown type %s, Usage: %s", args[0], cmd.Usage())
	}

	return nil
}
