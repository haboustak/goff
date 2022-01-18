package module

import (
	"bufio"
	"encoding/json"
	"fmt"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
	"golang.org/x/mod/sumdb"
	"io"
	"net/url"
	"path"
	"sort"
	"strings"
)

type moduleInfo struct {
	Version string
	Time    string
}

type Module struct {
	Path    string
	Version string
}

func New(mod module.Version) Module {
	var m Module
	m.Path = mod.Path
	m.Version = mod.Version

	return m
}

// Read a module  path and optional version from a string
func Parse(fullName string) (Module, error) {
	m := Module{
		Path:    fullName,
		Version: "",
	}

	// Handle version-specific reference: module@1.0.0
	if idx := strings.Index(fullName, "@"); idx > 0 {
		m.Path = fullName[:idx]
		m.Version = fullName[idx+1:]
	}

	return m, nil
}

// Escape special characters in module path strings
func (m Module) EscapedPath() string {
	path, _ := module.EscapePath(m.Path)
	return path
}

// Escape special characters in module version strings
func (m Module) EscapedVersion() string {
	version, _ := module.EscapeVersion(m.Version)
	return version
}

// Get the string representation of a module
func (m Module) String() string {
	return fmt.Sprintf("%s@%s", m.Path, m.Version)
}

// Create a new ModuleFile that represents a proxy's .info file
func (m Module) InfoFile() ModuleFile {
	return NewModuleFile(m, ModFileTypeInfo)
}

// Create a new ModuleFile that represents a proxy's .mod file
func (m Module) ModuleFile() ModuleFile {
	return NewModuleFile(m, ModFileTypeModule)
}

// Create a new ModuleFile that represents a proxy's .zip file
func (m Module) ZipFile() ModuleFile {
	return NewModuleFile(m, ModFileTypeZip)
}

// Get the most recent version of a module in semver order
func (m Module) LatestVersion(proxyUrl *url.URL) (string, error) {
	latestInfoPath := path.Join(m.EscapedPath(), "@latest")
	fileUrl, _ := proxyUrl.Parse(latestInfoPath)

	versionResp, err := HttpGet(fileUrl)
	if err != nil {
		return m.Version, err
	}
	defer versionResp.Close()

	infoBytes, err := io.ReadAll(versionResp)
	if err != nil {
		return m.Version, err
	}

	versionInfo := new(moduleInfo)
	if err := json.Unmarshal(infoBytes, versionInfo); err != nil {
		return m.Version, err
	}

	return versionInfo.Version, nil
}

// Download and validate the info, mod, and zip files from a module proxy
func (m Module) Download(proxyUrl *url.URL, outdir string, db *sumdb.Client) error {
	infoFile := m.InfoFile()
	err := infoFile.Download(proxyUrl, outdir, db)
	if err != nil {
		return fmt.Errorf("Failed to download module info: %v", err)
	}

	modFile := m.ModuleFile()
	err = modFile.Download(proxyUrl, outdir, db)
	if err != nil {
		return fmt.Errorf("Failed to download module mod: %v", err)
	}

	zipFile := m.ZipFile()
	err = zipFile.Download(proxyUrl, outdir, db)
	if err != nil {
		return fmt.Errorf("Failed to download module zip: %v", err)
	}

	return nil
}

// Get a list of available versions for a module from a proxy
func (m Module) Versions(proxyUrl *url.URL) ([]string, error) {
	versionListPath := path.Join(m.EscapedPath(), "@v", "list")
	versionUrl, _ := proxyUrl.Parse(versionListPath)

	versionResp, err := HttpGet(versionUrl)
	if err != nil {
		return nil, err
	}
	defer versionResp.Close()

	var versions []string
	scanner := bufio.NewScanner(versionResp)
	for scanner.Scan() {
		versions = append(versions, scanner.Text())
	}

	if scanner.Err() != nil {
		return nil, fmt.Errorf("Failed to parse version response from %v", versionUrl)
	}

	sort.Slice(versions, func(a, b int) bool {
		return semver.Compare(versions[a], versions[b]) > 0
	})

	return versions, nil
}

// Download the module file for a user-provided path and version
// and convert the path to the canonical form
func (m Module) CanonicalizePath(proxyUrl *url.URL) (string, error) {
	modFile := m.ModuleFile()
	modUrl, err := proxyUrl.Parse(modFile.ProxyPath)
	if err != nil {
		return m.Path, fmt.Errorf("Unable to build proxy Url: %v %v: %v", proxyUrl, modFile.ProxyPath, err)
	}

	modResp, err := HttpGet(modUrl)
	if err != nil {
		versionList, err := m.Versions(proxyUrl)
		if err != nil {
			return m.Path, fmt.Errorf("Proxy failed to return mod for %v: %v", m.Path, modUrl)
		}

		if len(versionList) == 0 {
			return m.Path, fmt.Errorf("Failed to find version %v for %v. Proxy list contained no tagged versions.", m.Version, m.Path)
		}

		versions := strings.Join(versionList, "\n  ")
		return m.Path, fmt.Errorf("Failed to find version %v for %v\nAvailable versions:\n  %v", m.Version, m.Path, versions)
	}
	defer modResp.Close()

	// Rewrite the module path based on the value in the mod file. Some modules
	// can be resolved using multiple urls. For example, github.com urls are
	// case-insensitive. This reduces duplicate module downloads.
	modFileBytes, err := io.ReadAll(modResp)
	if err != nil {
		return m.Path, err
	}

	modDetails, err := modfile.ParseLax(m.Path, modFileBytes, nil)
	if err != nil {
		return m.Path, fmt.Errorf("Failed to parse gomod file for %s: %v", m.String(), err)
	}

	return modDetails.Module.Mod.Path, nil
}
