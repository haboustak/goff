package main

import (
	"flag"
	"fmt"
	"os"
)

var BasePath string
var Version = "v0.9.1"

type Command struct {
	Name  string
	Run   func(cmd *Command) error
	Flags flag.FlagSet
	Usage string
}

var commands = []*Command{
	cmdServe,
	cmdDownload,
	cmdUpload,
}

func main() {
	var showHelp bool
	var version bool

	flag.BoolVar(&showHelp, "h", false, "show help")
	flag.BoolVar(&version, "v", false, "print version information")

	flag.Usage = func() {
		printUsage(defaultUsage)
	}
	flag.Parse()
	args := flag.Args()

	if showHelp && len(args) == 0 {
		printUsage(defaultUsage)
	} else if version {
		printVersion()
	}

	var command string
	if len(args) > 0 {
		command = args[0]
		args = args[1:]
	}

	cmdFound := false
	for _, cmd := range commands {
		if cmd.Name != command {
			continue
		}

		if showHelp {
			printUsage(cmd.Usage)
		}

		cmd.Flags.Usage = func() {
			printUsage(cmd.Usage)
		}
		cmd.Flags.Parse(args)
		err := cmd.Run(cmd)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		cmdFound = true
		break
	}

	if !cmdFound {
		if command != "" {
			fmt.Fprintf(os.Stderr, "Unknown operation \"%s\", try \"goff -h\"\n", command)
		} else {
			printUsage(defaultUsage)
		}
		os.Exit(1)
	}
}

var defaultUsage = `goff is a tool for managing an offline Go proxy

Usage:
    goff [-h] [-v] COMMAND [arguments]

Options:
   -h           show this help
   -v           print version information

Commands:
    serve       Start the proxy HTTP service
    download    Download a module and into a module set
    upload      Upload a module set to the proxy
`

func printUsage(usage string) {
	fmt.Fprintf(os.Stderr, usage)
	os.Exit(1)
}

func printVersion() {
	fmt.Fprintf(os.Stdout, "goff %s\n", Version)
	os.Exit(0)
}
