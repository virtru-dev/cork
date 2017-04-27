package main

import (
	"github.com/virtru/cork/server/environment"
	"gopkg.in/urfave/cli.v1"
)

func init() {
	command := cli.Command{
		Name:        "save-env",
		Description: "Saves the environment variables from docker to a json file. This is to support the ssh runner",
		Action:      cmdSaveEnv,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:   "output, o",
				Usage:  "The path to the output file",
				Value:  "/cork.env.json",
				EnvVar: "CORK_ENV_PATH",
			},
		},
	}
	registerCommand(command)
}

func cmdSaveEnv(c *cli.Context) error {
	return environment.SaveEnvFile(c.String("output"))
}
