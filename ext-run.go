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
			cli.StringFlag{
				Name:   "output, o",
				Usage:  "The path to the output destination",
				EnvVar: "CORK_OUTPUT_DESTINATION",
				Value:  "outputs.json",
			},
			cli.StringSliceFlag{
				Name:  "param, p",
				Usage: "Set Paramater param_name=param_value",
			},
		},
	}
	registerCommand(command)
}

func cmdExternalRun(c *cli.Context) error {
	corkType := c.Args().Get(0)
	stageName := c.Args().Get(1)

	if corkType == "" {
		return fmt.Errorf("Need a url for corkType")
	}

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
