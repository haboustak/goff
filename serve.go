package main

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
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

	modPath := strings.TrimSuffix(request.URL.Path, "/@v/list")[1:]
	versionGlob := path.Join(rootDir, modPath, "*.info")
	fmt.Printf("Searching %v\n", versionGlob)

	versionFiles, err := filepath.Glob(versionGlob)
	if err != nil || len(versionFiles) == 0 {
		http.Error(
			writer,
			fmt.Sprintf("Search failed %v: %v", versionGlob, err),
			http.StatusNotFound)
	}

	for _, versionFile := range versionFiles {
		version := strings.TrimSuffix(path.Base(versionFile), ".info")
		fmt.Fprintf(writer, "%v\n", version)
	}
}

// fixupProxyPath converts a goproxy-protocol compliant package path that contains '!' characters to indicate a capital letter follows to a
// path that does not contain '!' characters and matches the files on-disk in a case-sensitive filesystem
func fixupProxyPath(goProxyPath string) string {
	idx := 0
	outstr := ""
	for idx < len(goProxyPath) {
		if goProxyPath[idx] == '!' {
			// The following letter should be upper-cased
			outstr += strings.ToUpper(string(goProxyPath[idx+1]))
			idx += 1
		} else {
			outstr += string(goProxyPath[idx])
		}

		idx += 1
	}
	return outstr
}

func redirect(writer http.ResponseWriter, request *http.Request) {
	println(request.URL.Path)
	modValues := strings.Split(request.URL.Path, "/@v/")
	if len(modValues) == 1 {
		http.NotFound(writer, request)
		return
	}

	resourcePath := path.Join("/modules", modValues[0], modValues[1])
	// Fix up the goproxy-path to match the filesystem so nginx can serve the correct file
	resourcePath = fixupProxyPath(resourcePath)

	println(fmt.Sprintf("Redirecting to %v", resourcePath))
	writer.Header().Set("X-Accel-Redirect", resourcePath)
	fmt.Fprintf(writer, "")
}

func home(writer http.ResponseWriter, request *http.Request) {
	fmt.Fprintf(writer, "home home home")
}
