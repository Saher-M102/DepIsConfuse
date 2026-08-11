package DepIsConfuse

// Registry represents a package registry to check dependencies against.
type Registry string

const (
	NPM  Registry = "npm"
	PyPI Registry = "pypi"
)

// Dependency represents a single package name to be checked, tagged with
// the registry it should be verified against.
type Dependency struct {
	Name     string
	Registry Registry
}

// Status is the outcome classification of checking a single dependency.
type Status int

const (
	// StatusClaimed: the name exists on the public registry (safe).
	StatusClaimed Status = iota
	// StatusUnclaimed: the name does not exist on the public registry 
	// a real dependency confusion risk, since anyone could publish it.
	StatusUnclaimed
	// StatusScopeProtected means the package name is unclaimed, but its npm
// scope is already registered. Only members of the scope can publish
// packages under it, so the name is not available to an outside user.
	StatusScopeProtected
	// StatusSkipped: the dependency matched a user-provided --safe
	// pattern and was not checked against the registry at all.
	StatusSkipped
	// StatusError means the dependency could not be verified because the
// registry check failed, such as from a network error or unexpected response.
	StatusError
)

// Result is the outcome of checking a single dependency against a registry.
type Result struct {
	Dependency Dependency
	Status     Status
	Err        error // set when Status == StatusError
}
