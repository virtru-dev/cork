package main

import (
	"fmt"

	"gopkg.in/urfave/cli.v1"
)

func init() {
	command := cli.Command{
		Name:        "init",
		Description: "Initialize a project using a specific project type",
		Action:      cmdInitialize,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:   "force-pull-image",
				Usage:  "Forces cork to pull the latest version of the cork container",
				EnvVar: "CORK_FORCE_PULL_IMAGE",
			},
		},
	}
	registerCommand(command)
}

func cmdInitialize(c *cli.Context) error {
	corkType := c.Args().Get(0)

	if corkType == "" {
		return fmt.Errorf("Must specify corkType")
	}

	corkDef := &CorkDefinition{
		Type: corkType,
	}

	err := corkDef.LoadName()
	if err != nil {
		return err
	}

	fmt.Println("Initialize does not yet do anything")
	return nil
}
