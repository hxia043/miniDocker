package main

import (
	"docker/internal/runc/cmd"
	"os"

	log "github.com/Sirupsen/logrus"

	"github.com/urfave/cli"
)

const (
	name  = "minidocker"
	usage = `minidocker is a simple container runtime implementation.`
)

func createApp() *cli.App {
	app := cli.NewApp()
	app.Name = name
	app.Usage = usage

	app.Commands = []cli.Command{
		cmd.InitCommand,
		cmd.RunCommand,
		cmd.CommitCommand,
		cmd.ListCommand,
		cmd.LogCommand,
		cmd.StopCommand,
		cmd.RemoveCommand,
		cmd.ExecCommand,
		cmd.NetworkCommand,
	}

	app.Before = func(ctx *cli.Context) error {
		log.SetFormatter(&log.JSONFormatter{})
		log.SetOutput(os.Stdout)
		return nil
	}

	return app
}

func main() {
	app := createApp()
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
