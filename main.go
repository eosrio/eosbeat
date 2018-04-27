package main

import (
	"os"
	"github.com/eosrio/eosbeat/cmd"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
