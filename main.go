package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	var (
		showVersion = flag.Bool("version", false, "Show version information")
		help        = flag.Bool("help", false, "Show help")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("{{PROJECT_NAME}} %s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}

	if *help {
		printUsage()
		os.Exit(0)
	}

	log.Printf("Starting {{PROJECT_NAME}}...")

	// TODO: Add your application logic here
	fmt.Println("Hello, World!")

	log.Printf("{{PROJECT_NAME}} finished successfully")
}

func printUsage() {
	fmt.Printf(`Usage: {{PROJECT_NAME}} [OPTIONS]

A Go application template.

Options:
  -version    Show version information
  -help       Show this help message

Examples:
  {{PROJECT_NAME}}              # Run the application
  {{PROJECT_NAME}} -version     # Show version
  {{PROJECT_NAME}} -help        # Show help

`)
}
