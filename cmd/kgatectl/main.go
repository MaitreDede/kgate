package main

import (
	"flag"
	"log"
	"os"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Command = cobra.Command

var (
	namespace string

	getOpts = metav1.GetOptions{}
)

func main() {
	cmd := &Command{
		Use: os.Args[0],
	}

	pflags := cmd.PersistentFlags()
	flag.VisitAll(pflags.AddGoFlag)

	pflags.StringVarP(&namespace, "namespace", "n", "", "Namespace to use")

	cmd.AddCommand(
		initCommand(),
		exposeRemoteCommand(),
	)

	cmd.PersistentPreRun = func(cmd *Command, args []string) {
		if namespace == "" {
			log.Fatal("Namespace is mandatory")
		}
	}

	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
