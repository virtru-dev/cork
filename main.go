package main

import (
	"io/ioutil"
	"os"

	log "github.com/Sirupsen/logrus"
	"google.golang.org/grpc/grpclog"
	"gopkg.in/urfave/cli.v1"
)

// Commands
var Commands []cli.Command

func setupApp() *cli.App {
	app := cli.NewApp()

	app.Usage = "<command> [subcommand] [options...] [args...]"
	app.Description = "cork - A container workflow tool"

	cli.HelpFlag = cli.BoolFlag{
		Name:  "help, h",
		Usage: "show help",
	}

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug",
			Usage: "Set debug",
		},
	}
	app.Version = Version

	app.Before = func(c *cli.Context) error {
		grpclog.SetLogger(log.StandardLogger())
		if c.Bool("debug") {
			log.SetLevel(log.DebugLevel)
			log.Debug("Debug on")
			err := os.Setenv("CORK_DEBUG", "true")
			if err != nil {
				return err
			}
		} else {
			log.SetOutput(ioutil.Discard)
		}
		return nil
	}

	app.Commands = Commands
	return app
}

func registerCommand(cmd cli.Command) {
	Commands = append(Commands, cmd)
}

func main() {
	app := setupApp()
	app.Run(os.Args)
}
