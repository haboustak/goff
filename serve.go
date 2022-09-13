package main

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/mod/module"
)

var (
	bind    string
	rootDir string
)

var cmdServe = &Command{
	Name: "serve",
	Run:  serve,
	Usage: `Usage:
    goff [-h] serve [-bind IP:PORT] ROOT_DIR

Start the HTTP proxy with modules stored in ROOT_DIR

Options:
    -bind   Set the IP and port used by the HTTP server (default=localhost:5000)
    -h      show this help
`,
}

func init() {
	cmdServe.Flags.StringVar(&bind, "bind", "localhost:5000", "Set the IP and port used by the HTTP server (default=localhost:5000)")
}

func serve(self *Command) error {
	args := self.Flags.Args()
	if len(args) != 1 {
		return fmt.Errorf("You must provide the root directory of the package storage")
	}

	rootDir = args[0]
	rootInfo, err := os.Stat(rootDir)
	if os.IsNotExist(err) {
		return fmt.Errorf("The path \"%v\" does not exist.", rootDir)
	}

	if !rootInfo.IsDir() {
		return fmt.Errorf("The path \"%v\" is not a directory.", rootDir)
	}

	fmt.Printf("Serving modules from %v\n", rootDir)
	http.HandleFunc("/", router)
	http.ListenAndServe(bind, nil)

	return nil
}

func router(writer http.ResponseWriter, request *http.Request) {
	requestPath := request.URL.Path
	if strings.HasSuffix(requestPath, "/@v/list") {
		list(writer, request)
	} else if requestPath == "/" {
		home(writer, request)
	} else {
		redirect(writer, request)
	}
}

func list(writer http.ResponseWriter, request *http.Request) {
	println(request.URL.Path)

	escapedModPath := strings.TrimSuffix(request.URL.Path, "/@v/list")[1:]
	modPath, err := parseModPath(escapedModPath)
	if err != nil {
		http.Error(
			writer,
			fmt.Sprintf("Unable to list module: %v", err),
			http.StatusNotFound)
		return
	}
	versionGlob := path.Join(rootDir, modPath, "*.info")
	fmt.Printf("Searching %v\n", versionGlob)

	versionFiles, err := filepath.Glob(versionGlob)
	if err != nil || len(versionFiles) == 0 {
		http.Error(
			writer,
			fmt.Sprintf("Search failed %v: %v", versionGlob, err),
			http.StatusNotFound)
		return
	}

	for _, versionFile := range versionFiles {
		version := strings.TrimSuffix(path.Base(versionFile), ".info")
		fmt.Fprintf(writer, "%v\n", version)
	}
}

func redirect(writer http.ResponseWriter, request *http.Request) {
	println(request.URL.Path)
	modValues := strings.Split(request.URL.Path, "/@v/")
	if len(modValues) == 1 {
		http.NotFound(writer, request)
		return
	}
	modPath, err := parseModPath(modValues[0][1:])
	if err != nil {
		http.Error(
			writer,
			fmt.Sprintf("Unable to access module: %v", err),
			http.StatusNotFound)
		return
	}
	version := modValues[1]
	if modPath != modValues[0] {
		// If the mod path was unescaped we may need to unescape the version
		version, err = module.UnescapeVersion(version)
		if err != nil {
			http.Error(
				writer,
				fmt.Sprintf("Invalid version: %v", err),
				http.StatusNotFound)
			return
		}
	}

	resourcePath := path.Join("/modules", modPath, version)
	println(fmt.Sprintf("Redirecting to %v", resourcePath))
	writer.Header().Set("X-Accel-Redirect", resourcePath)
	fmt.Fprintf(writer, "")
}

func home(writer http.ResponseWriter, request *http.Request) {
	fmt.Fprintf(writer, "home home home")
}

// parseModPath will first unescape the module and check if it exists. If it
// exists it will return the unescaped module. If it does not exist it will
// then check to see if the escaped module exists (in the case we are running
// on a case insensitive filesystem). If the escaped path exists it will return
// the escaped path. If that is not the case, or the module path is invalid,
// then an error will be returned.
func parseModPath(modPath string) (string, error) {
	fmt.Printf("Parse %s\n", modPath)
	unescaped, err := module.UnescapePath(modPath)
	if err != nil {
		return "", err
	}
	if isDir(path.Join(rootDir, unescaped)) {
		return unescaped, nil
	}
	if modPath != unescaped && isDir(path.Join(rootDir, modPath)) {
		return modPath, nil
	}
	return "", fmt.Errorf("module %s does not exist", unescaped)
}

func isDir(path string) bool {
	stat, err := os.Stat(path)
	return err == nil && stat.IsDir()
}
