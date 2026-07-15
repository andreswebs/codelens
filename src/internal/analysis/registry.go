package analysis

import (
	"fmt"
	"sort"
	"sync"
)

// The registry is a process-global set of descriptors keyed by canonical name,
// with a secondary index mapping every name and alias to its canonical name.
// Analyses register themselves from init functions, so registration happens
// before any command runs; the mutex guards against registration racing a
// concurrent Lookup or All in tests.
var (
	mu          sync.RWMutex
	descriptors = map[string]Descriptor{} // canonical name -> descriptor
	byKey       = map[string]string{}     // name or alias -> canonical name
)

// Register adds d to the registry, indexing its canonical name and every alias.
// A collision with an already-registered name or alias, or a name/alias
// duplicated within d itself, is a programmer error and panics; registration is
// expected at init time where a panic surfaces the mistake immediately.
func Register(d Descriptor) {
	mu.Lock()
	defer mu.Unlock()

	keys := make([]string, 0, 1+len(d.Aliases))
	keys = append(keys, d.Name)
	keys = append(keys, d.Aliases...)

	seen := make(map[string]bool, len(keys))
	for _, k := range keys {
		if existing, dup := byKey[k]; dup {
			panic(fmt.Sprintf("analysis: key %q already registered for %q", k, existing))
		}
		if seen[k] {
			panic(fmt.Sprintf("analysis: key %q duplicated within descriptor %q", k, d.Name))
		}
		seen[k] = true
	}

	descriptors[d.Name] = d
	for _, k := range keys {
		byKey[k] = d.Name
	}
}

// Lookup resolves a canonical name or alias to its descriptor, reporting
// whether one was registered.
func Lookup(nameOrAlias string) (Descriptor, bool) {
	mu.RLock()
	defer mu.RUnlock()

	name, ok := byKey[nameOrAlias]
	if !ok {
		return Descriptor{}, false
	}
	return descriptors[name], true
}

// All returns every registered descriptor sorted by canonical name. The result
// is a fresh slice, so callers may reorder or replace elements without
// affecting the registry.
func All() []Descriptor {
	mu.RLock()
	defer mu.RUnlock()

	out := make([]Descriptor, 0, len(descriptors))
	for _, d := range descriptors {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
