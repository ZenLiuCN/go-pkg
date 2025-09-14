package commands

import (
	. "github.com/urfave/cli/v3"
	"log"
)

func Commands() *Command {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	return &Command{
		Name:  "units",
		Usage: "units for npm and maven",
		Commands: []*Command{
			npm(),
			tsd(),
			mvn(),
			httpd(),
			esbuild(),
		},
	}
}
