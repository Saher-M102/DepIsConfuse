package registry

import (
	"fmt"
	"net/http"
	"path"
	"sync"

	DepIsConfuse  "github.com/Saher-M102/DepIsConfuse/internal"
)

/* MatchesSafePattern checks whether a dependency name matches one of the
 configured safe patterns. Matching dependencies are skipped from registry
 checks because they are explicitly marked as safe by the user.*/

func MatchesSafePattern(name string, patterns []string) bool {
	for _, p := range patterns {
		if ok, err := path.Match(p, name); err == nil && ok {
			return true
		}
	}
	return false
}


// CheckAll checks all dependencies using the configured concurrency limit.
// Dependencies matching a safe pattern are skipped without making a
// registry request.


func CheckAll(client *http.Client, deps []DepIsConfuse.Dependency, concurrency int, safePatterns []string) []DepIsConfuse.Result {
	if concurrency < 1 {
		concurrency = 1
	}

	results := make([]DepIsConfuse.Result, len(deps))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i, dep := range deps {
		if MatchesSafePattern(dep.Name, safePatterns) {
			results[i] = DepIsConfuse.Result{Dependency: dep, Status: DepIsConfuse.StatusSkipped}
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(i int, dep DepIsConfuse.Dependency) {
			defer wg.Done()
			defer func() { <-sem }()
			results[i] = checkOne(client, dep)
		}(i, dep)
	}

	wg.Wait()
	return results
}

// checkOne runs the registry specific check for a dependency.
func checkOne(client *http.Client, dep DepIsConfuse.Dependency) DepIsConfuse.Result {
	switch dep.Registry {
	case DepIsConfuse.NPM:
		return checkOneNPM(client, dep)
	case DepIsConfuse.PyPI:
		claimed, err := CheckPyPI(client, dep.Name)
		return toResult(dep, claimed, err)
	default:
		return DepIsConfuse.Result{Dependency: dep, Status: DepIsConfuse.StatusError, Err: fmt.Errorf("unknown registry %q", dep.Registry)}
	}
}

func checkOneNPM(client *http.Client, dep DepIsConfuse.Dependency) DepIsConfuse.Result {
	claimed, err := CheckNPM(client, dep.Name)
	if err != nil {
		return DepIsConfuse.Result{Dependency: dep, Status: DepIsConfuse.StatusError, Err: err}
	}
	if claimed {
		return DepIsConfuse.Result{Dependency: dep, Status: DepIsConfuse.StatusClaimed}
	}
	if !IsScoped(dep.Name) {
		return DepIsConfuse.Result{Dependency: dep, Status: DepIsConfuse.StatusUnclaimed}
	}

	// A scoped package is only potentially available if the scope itself
// is not already registered.
	scope := ScopeOf(dep.Name)
	scopeRegistered, scopeErr := CheckNPMScope(client, scope)
	if scopeErr != nil {
	// If the scope check fails, return an error instead of treating the
// package as unclaimed.
		return DepIsConfuse.Result{
			Dependency: dep,
			Status:     DepIsConfuse.StatusError,
			Err:        scopeErr,
		}
	}
	if scopeRegistered {
		return DepIsConfuse.Result{Dependency: dep, Status: DepIsConfuse.StatusScopeProtected}
	}
	return DepIsConfuse.Result{Dependency: dep, Status: DepIsConfuse.StatusUnclaimed}
}

func toResult(dep DepIsConfuse.Dependency, claimed bool, err error) DepIsConfuse.Result {
	if err != nil {
		return DepIsConfuse.Result{Dependency: dep, Status: DepIsConfuse.StatusError, Err: err}
	}
	if claimed {
		return DepIsConfuse.Result{Dependency: dep, Status: DepIsConfuse.StatusClaimed}
	}
	return DepIsConfuse.Result{Dependency: dep, Status: DepIsConfuse.StatusUnclaimed}
}
