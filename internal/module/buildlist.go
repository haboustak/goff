package module

import (
	"golang.org/x/mod/semver"
	"sync"
)

// A list of modules required for a given build
type BuildList struct {
	modules map[Module]bool
	sync.Mutex
}

func NewBuildList() *BuildList {
	b := &BuildList{
		modules: make(map[Module]bool),
	}

	return b
}

// Add a module to the build list if it has not been seen before
//
// returns true for new modules, returns false if the module has already been visited
func (b *BuildList) Visit(m Module) bool {
	b.Lock()
	defer b.Unlock()

	if _, visited := b.modules[m]; visited {
		return false
	}

	b.modules[m] = true
	return true
}

// Returns a list of all modules in the BuildList
func (b *BuildList) All() []Module {
	b.Lock()
	defer b.Unlock()

	latest := make(map[string]Module)
	for m, _ := range b.modules {
		latestVersion, ok := latest[m.Path]
		if !ok || semver.Compare(m.Version, latestVersion.Version) > 0 {
			latest[m.Path] = m
		}
	}

	list := make([]Module, 0, len(latest))
	for _, m := range latest {
		list = append(list, m)
	}

	return list
}
