package main

import (
	"context"
	"log"
	"os"

	"github.com/github/codeql-action-sync/cmd"
)

func main() {
	ctx := context.Background()
	if err := cmd.Execute(ctx); err != nil {
		if err == cmd.SilentErr {
			os.Exit(1)
		}
		log.Fatalf("%+v", err)
	}
}
