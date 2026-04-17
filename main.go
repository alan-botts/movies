package main

import (
	"os"

	"github.com/alan-botts/movies/cmd/movies"
)

func main() {
	if err := movies.Execute(); err != nil {
		os.Exit(1)
	}
}
