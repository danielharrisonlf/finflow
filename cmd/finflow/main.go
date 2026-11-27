package main

import (
	"finflow/internal/cli"
	"finflow/internal/config"
)

func main() {
	config.LoadDotEnv()
	cli.Execute()
}
