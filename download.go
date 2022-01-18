package main

import (
	"fmt"
	"github.com/haboustak/goff/internal/module"
	"github.com/haboustak/goff/internal/task"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/sumdb"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

var (
	outDir        string
	downloadProxy string
)

var cmdDownload = &Command{
	Name: "download",
	Run:  download,
	Usage: `Usage:
    goff [-h] download [-outdir path] [-proxy hostname] modules

Download modules and collect them into a module set

Options:
    -h      show this help
    -outdir directory where modules will be stored (default=./modules)
    -proxy  hostname of proxy to download modules from (default=go env GOPROXY)
`,
}

type downloadResult struct {
	Module module.Module
	Error  error
}

type downloadRequest struct {
	Proxy      *url.URL
	OutDir     string
	BuildList  *module.BuildList
	Queue      *task.TaskQueue
	SumDb      *sumdb.Client
	Downloaded chan downloadResult
}

func init() {
	var proxyHost string

	result, err := exec.Command("go", "env", "GOPROXY").Output()
	if err != nil {
		proxyHost = "proxy.golang.org"
	} else {
		proxyHost = strings.SplitN(string(result), ",", 2)[0]
	}

	cmdDownload.Flags.StringVar(&outDir, "outdir", "modules", "module set output directory")
	cmdDownload.Flags.StringVar(&downloadProxy, "proxy", proxyHost, "hostname of module proxy")
}

func proxyUrl(hostname string) *url.URL {
	host, err := url.Parse(hostname)
	if err != nil || (host.Scheme != "https" && host.Scheme != "http") {
		// Remove a scheme prefix, if present
		nameParts := strings.SplitN(hostname, "//", 2)
		if len(nameParts) > 1 {
			hostname = nameParts[1]
		}

		host, err = url.Parse("https://" + hostname)
		if err != nil {
			host, _ = url.Parse("https://proxy.golang.org")
		}
	}

	return host
}

func updateBuildList(req *downloadRequest, m module.Module) error {
	// only download a module once
	if !req.BuildList.Visit(m) {
		return nil
	}

	modFile := m.ModuleFile()
	err := modFile.Download(req.Proxy, req.OutDir, req.SumDb)
	if err != nil {
		return fmt.Errorf("Failed to download %s gomod file: %v", m.String(), err)
	}

	modFilePath := path.Join(req.OutDir, modFile.FilePath)
	modFileBytes, err := os.ReadFile(modFilePath)
	if err != nil {
		return fmt.Errorf("Failed to read file %s", modFilePath)
	}

	// Visit all dependencies listed in the go.mod file
	modDetails, err := modfile.ParseLax(m.Path, modFileBytes, nil)
	if err != nil {
		os.Remove(modFilePath)
		return fmt.Errorf("Failed to parse gomod file for %s: %v", m.String(), err)
	}

	for _, r := range modDetails.Require {
		depModule := module.New(r.Mod)
		req.Queue.Append(func() error {
			return updateBuildList(req, depModule)
		})
	}

	return nil
}

func download(self *Command) error {
	moduleNames := self.Flags.Args()
	if len(moduleNames) < 1 {
		return fmt.Errorf("You must provide one or more modules to download")
	}

	for _, name := range moduleNames {
		m, err := module.Parse(name)
		if err != nil {
			return err
		}

		req := new(downloadRequest)
		req.OutDir = outDir
		req.Proxy = proxyUrl(downloadProxy)
		req.Queue = task.NewTaskQueue(0)
		req.Downloaded = make(chan downloadResult)

		dbUrl, _ := url.Parse("https://sum.golang.org")
		req.SumDb = module.NewClient(dbUrl)

		// Use the latest version if a specific version was not specified
		if m.Version == "" {
			m.Version, err = m.LatestVersion(req.Proxy)
			if err != nil {
				return fmt.Errorf("Failed to get latest version for module %v: %v\n", m.String(), err)
			}
		}

		// Get the true capitalization of this module's path from the proxy
		// Modules can be downloaded from GitHub using any combination of upper-
		// and lower-case letters. We want to minimize case-only variants.
		if m.Path, err = m.CanonicalizePath(req.Proxy); err != nil {
			return err
		}

		fmt.Printf("Collecting requirements for %v\n", m)

		// recursively build the list of modules required to build this module
		req.BuildList = module.NewBuildList()
		req.Queue.Append(func() error {
			return updateBuildList(req, m)
		})
		<-req.Queue.Wait()
		if req.Queue.LastError != nil {
			return req.Queue.LastError
		}

		deps := req.BuildList.All()
		nDeps := len(deps)

		statusDone := make(chan struct{})
		go func() {
			next := 0
			for result := range req.Downloaded {
				next++
				fmt.Printf("%v/%v: %v\n", next, nDeps, result.Module)
				if result.Error != nil {
					fmt.Println(result.Error)
				}
			}
			close(statusDone)
		}()

		// download all required modules
		for _, dependency := range deps {
			mod := dependency
			req.Queue.Append(func() error {
				err := mod.Download(req.Proxy, req.OutDir, req.SumDb)
				req.Downloaded <- downloadResult{mod, err}
				return err
			})
		}
		<-req.Queue.Wait()
		close(req.Downloaded)
		<-statusDone

		if req.Queue.LastError != nil {
			return fmt.Errorf("One or more modules failed to download")
		}

		pathPrefix := ""
		if !filepath.IsAbs(outDir) {
			pathPrefix = "./"
		}
		modSuffix := ""
		if nDeps > 1 {
			modSuffix = "s"
		}

		fmt.Printf("Downloaded %v module%v to %v%v\n", nDeps, modSuffix, pathPrefix, req.OutDir)
	}

	return nil
}
