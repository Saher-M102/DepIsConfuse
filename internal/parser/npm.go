package parser

import (
	"encoding/json"
	"fmt"
	"sort"

	DepIsConfuse "github.com/Saher-M102/DepIsConfuse/internal"
)


// packageJSON mirrors the fields of package.json we care about. All four
// dependency sections are considered because a dependency confusion attack
// can target any of them, not just "dependencies".
type packageJSON struct {
	Dependencies         map[string]string `json:"dependencies"`
	DevDependencies      map[string]string `json:"devDependencies"`
	PeerDependencies     map[string]string `json:"peerDependencies"`
	OptionalDependencies map[string]string `json:"optionalDependencies"`
}

// ParsePackageJSON extracts unique dependency names from package.json
// content and tags them as npm dependencies.
func ParsePackageJSON(data []byte) ([]DepIsConfuse.Dependency, error) {
	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("failed to parse package.json: %w", err)
	}

	seen := make(map[string]struct{})
	addAll := func(m map[string]string) {
		for name := range m {
			seen[name] = struct{}{}
		}
	}
	addAll(pkg.Dependencies)
	addAll(pkg.DevDependencies)
	addAll(pkg.PeerDependencies)
	addAll(pkg.OptionalDependencies)

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)

	deps := make([]DepIsConfuse.Dependency, 0, len(names))
	for _, name := range names {
		deps = append(deps, DepIsConfuse.Dependency{Name: name, Registry: DepIsConfuse.NPM})
	}
	return deps, nil
}
