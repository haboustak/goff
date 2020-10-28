package main

import (
	"fmt"
)

var (
	outDir string
)

var cmdDownload = &Command{
	Name: "download",
	Run:  download,
	Usage: `Usage:
    goff [-h] download [-outdir path] modules

Download modules and collect them into a module set

Options:
    -h      show this help
    -outdir directory where modules will be stored (default=./modules)
`,
}

func init() {
	cmdDownload.Flags.StringVar(&outDir, "outdir", "modules", "module set output directory")
}

func download(self *Command) error {
	args := self.Flags.Args()
	if len(args) != 2 {
		return fmt.Errorf("You must provide one or more modules to download")
	}

	return nil
}
