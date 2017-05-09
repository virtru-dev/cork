package main

import (
	"fmt"

	"gopkg.in/urfave/cli.v1"
)

func init() {
	command := cli.Command{
		Name:        "ext-run",
		Description: "External run",
		Action:      cmdExternalRun,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:   "force-pull-image",
				Usage:  "Forces cork to pull the latest version of the cork container",
				EnvVar: "CORK_FORCE_PULL_IMAGE",
			},
			cli.StringFlag{
				Name:   "ssh-key",
				Usage:  "The ssh key path to use",
				EnvVar: "CORK_SSH_KEY",
			},
		},
	}
	registerCommand(command)
}

func cmdExternalRun(c *cli.Context) error {
	corkType := c.Args().Get(0)
	stageName := c.Args().Get(1)

	if stageName == "" {
		return fmt.Errorf("stageName is required")
	}

	corkDef := &CorkDefinition{
		Type: corkType,
	}

	err := corkDef.LoadName()
	if err != nil {
		return err
	}

	err = executeCorkRun(c, corkDef, stageName)
	if err != nil {
		return err
	}
	return nil
}
