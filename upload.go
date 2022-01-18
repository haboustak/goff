package main

import (
	"fmt"
)

var (
	proxy string
)

var cmdUpload = &Command{
	Name: "upload",
	Run:  upload,
	Usage: `Usage:
    goff [-h] upload [-proxy URI] module_dir

Upload modules from module_dir to the Go proxy

Options:
    -h      show this help
    -proxy  proxy to upload modules to (default=$GOPROXY)
`,
}

func init() {
	cmdUpload.Flags.StringVar(&proxy, "proxy", "", "proxy to upload modules to")
}

func upload(self *Command) error {
	args := self.Flags.Args()
	if len(args) == 0 {
		return fmt.Errorf("You must specify the module set directory")
	}

	return nil
}
