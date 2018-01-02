package main

import (
	"log"

	"github.com/spf13/cobra"

	"github.com/mcluseau/kgate/client"
	"github.com/mcluseau/kgate/server"
)

func main() {
	cmd := &cobra.Command{}
	cmd.AddCommand(
		server.Command(),
		client.Command())

	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
