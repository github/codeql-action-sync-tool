package main

import (
	"context"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/github/codeql-action-sync/cmd"
)

func main() {
	log.SetLevel(log.DebugLevel)
	ctx := context.Background()
	if err := cmd.Execute(ctx); err != nil {
		if err == cmd.SilentErr {
			os.Exit(1)
		}
		log.Fatalf("%+v", err)
	}
}
