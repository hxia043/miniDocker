package main

import (
	"docker/internal/runc/cmd"
	"os"

	log "github.com/Sirupsen/logrus"

	"github.com/urfave/cli"
)

const usage = `minidocker is a simple container runtime implementation.`

func main() {
	app := cli.NewApp()
	app.Name = "minidocker"
	app.Usage = usage

	app.Commands = []cli.Command{cmd.InitCommand, cmd.RunCommand}

	app.Before = func(ctx *cli.Context) error {
		log.SetFormatter(&log.JSONFormatter{})
		log.SetOutput(os.Stdout)
		return nil
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
