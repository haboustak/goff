package module

import (
	"fmt"
	"golang.org/x/mod/sumdb"
	"golang.org/x/mod/sumdb/dirhash"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
)

type ModuleFileType string

// Types of files used by go proxies
const (
	ModFileTypeInfo   ModuleFileType = ".info"
	ModFileTypeModule ModuleFileType = ".mod"
	ModFileTypeZip    ModuleFileType = ".zip"
)

type ModuleFile struct {
	Mod       Module
	Type      ModuleFileType
	FileName  string
	FilePath  string
	ProxyPath string
}

// Create a new ModuleFile of the provided type for a Module
func NewModuleFile(m Module, fileType ModuleFileType) ModuleFile {
	var file ModuleFile

	if m.Version == "" {
		panic("Cannot create a ModuleFile for a Module without a Version")
	}

	file.Mod = m
	file.Type = fileType
	file.FileName = m.EscapedVersion() + string(fileType)
	file.FilePath = path.Join(m.EscapedPath(), file.FileName)
	file.ProxyPath = path.Join(m.EscapedPath(), "@v", file.FileName)

	return file
}

// Download a ModuleFile from a proxy
func (f ModuleFile) Download(proxyUrl *url.URL, outdir string, db *sumdb.Client) error {
	filePath := path.Join(outdir, f.FilePath)
	fileUrl, err := proxyUrl.Parse(f.ProxyPath)
	if err != nil {
		return fmt.Errorf("Failed to build proxy Url: %v %v", proxyUrl, f.ProxyPath)
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("Failed to create output directory for %v: %v", filePath, err)
	}

	out, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644)
	if os.IsExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("Failed to create destination file %v for %v: %v", filePath, fileUrl, err)
	}

	body, err := HttpGet(fileUrl)
	if err != nil {
		out.Close()
		os.Remove(filePath)
		return err
	}
	defer body.Close()

	if _, err := io.Copy(out, body); err != nil {
		out.Close()
		os.Remove(filePath)
		return fmt.Errorf("Could not download %v: %v", fileUrl, err)
	}

	if err := out.Close(); err != nil {
		os.Remove(filePath)
		return fmt.Errorf("Error closing %v: %v", fileUrl, err)
	}

	if err := f.checkSumDb(outdir, db); err != nil {
		os.Remove(filePath)
		return fmt.Errorf("Error validating file %v: %v", fileUrl, err)
	}

	return nil
}

func (f ModuleFile) getFileHash(outdir string) (string, string, error) {
	filePath := path.Join(outdir, f.FilePath)

	if f.Type == ModFileTypeModule {
		hashVersion := f.Mod.Version + "/go.mod"
		hash, err := dirhash.Hash1([]string{"go.mod"}, func(string) (io.ReadCloser, error) {
			return os.Open(filePath)
		})
		return hashVersion, hash, err
	} else if f.Type == ModFileTypeZip {
		hashVersion := f.Mod.Version
		hash, err := dirhash.HashZip(filePath, dirhash.DefaultHash)
		return hashVersion, hash, err
	}

	return "", "", nil
}

func (f ModuleFile) checkSumDb(outdir string, db *sumdb.Client) error {
	hashVersion, hash, err := f.getFileHash(outdir)
	if err != nil {
		return fmt.Errorf("Failed to hash mod file bytes: %v", err)
	}

	// some files do not have checksums
	if hash == "" {
		return nil
	}

	expected := fmt.Sprintf("%s %s %s", f.Mod.Path, hashVersion, hash)
	lines, err := db.Lookup(f.Mod.Path, hashVersion)
	for _, line := range lines {
		if line == expected {
			return nil
		}
	}

	return fmt.Errorf("Module hash %s not found: %s", expected, lines)
}
