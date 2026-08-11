package parser

import (
	"sort"
	"testing"

	DepIsConfuse "github.com/Saher-M102/DepIsConfuse/internal"
)

func namesOf(deps []DepIsConfuse.Dependency) []string {
	names := make([]string, len(deps))
	for i, d := range deps {
		names[i] = d.Name
	}
	sort.Strings(names)
	return names
}

func assertNames(t *testing.T, got []DepIsConfuse.Dependency, want []string) {
	t.Helper()
	gotNames := namesOf(got)
	sort.Strings(want)
	if len(gotNames) != len(want) {
		t.Fatalf("got %d deps %v, want %d deps %v", len(gotNames), gotNames, len(want), want)
	}
	for i := range gotNames {
		if gotNames[i] != want[i] {
			t.Fatalf("got %v, want %v", gotNames, want)
		}
	}
}

func TestParsePyprojectToml_PEP621Basic(t *testing.T) {
	data := []byte(`
[project]
name = "demo"
authors = [{name = "Someone", email = "someone@example.com"}]
keywords = ["cli", "tool"]
dependencies = [
  "requests>=2.28",
  "click",
]
`)
	deps, err := ParsePyprojectToml(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// authors/keywords must NOT leak into the dependency list.
	assertNames(t, deps, []string{"requests", "click"})
}

func TestParsePyprojectToml_OptionalDependencies(t *testing.T) {
	data := []byte(`
[project]
dependencies = ["requests"]

[project.optional-dependencies]
dev = ["pytest>=7.0", "black"]
docs = ["sphinx"]
`)
	deps, err := ParsePyprojectToml(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertNames(t, deps, []string{"requests", "pytest", "black", "sphinx"})
}

func TestParsePyprojectToml_PoetryStyle(t *testing.T) {
	data := []byte(`
[tool.poetry.dependencies]
python = "^3.9"
django = "^4.2"
requests = "^2.28"

[tool.poetry.group.dev.dependencies]
pytest = "^7.0"

[tool.poetry.dev-dependencies]
black = "^23.0"
`)
	deps, err := ParsePyprojectToml(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "python" itself must be excluded  it's a version constraint, not a package.
	assertNames(t, deps, []string{"django", "requests", "pytest", "black"})
}

func TestParsePyprojectToml_MixedPEP621AndPoetry(t *testing.T) {
	data := []byte(`
[project]
name = "demo"
dependencies = [
  "requests>=2.28",
  "click",
  "this-toml-pkg-does-not-exist-xyz-999",
]

[project.optional-dependencies]
dev = ["pytest>=7.0"]

[tool.poetry.dependencies]
python = "^3.9"
django = "^4.2"
`)
	deps, err := ParsePyprojectToml(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertNames(t, deps, []string{"requests", "click", "this-toml-pkg-does-not-exist-xyz-999", "pytest", "django"})
}

func TestParsePyprojectToml_SingleLineArray(t *testing.T) {
	data := []byte(`
[project]
dependencies = ["requests", "click", "flask"]
`)
	deps, err := ParsePyprojectToml(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertNames(t, deps, []string{"requests", "click", "flask"})
}

func TestParseRequirementsTxt(t *testing.T) {
	data := []byte(`
# a comment
requests==2.28.1
flask>=2.0,<3.0

this-pkg-does-not-exist-abc-123456789
numpy[extra]==1.24.0; python_version >= "3.8"
-r other-requirements.txt
-e .
git+https://github.com/some/repo.git#egg=somepkg
`)
	deps, err := ParseRequirementsTxt(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertNames(t, deps, []string{"requests", "flask", "this-pkg-does-not-exist-abc-123456789", "numpy"})
}

func TestParsePackageJSON_AllSections(t *testing.T) {
	data := []byte(`{
  "dependencies": {"express": "^4.18.0"},
  "devDependencies": {"jest": "^29.0.0"},
  "peerDependencies": {"react": "^18.0.0"},
  "optionalDependencies": {"fsevents": "^2.3.0"}
}`)
	deps, err := ParsePackageJSON(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertNames(t, deps, []string{"express", "jest", "react", "fsevents"})
}

func TestParsePackageJSON_ScopedPackages(t *testing.T) {
	data := []byte(`{
  "dependencies": {"@babel/core": "^7.0.0", "@myorg/internal-lib": "^1.0.0"}
}`)
	deps, err := ParsePackageJSON(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertNames(t, deps, []string{"@babel/core", "@myorg/internal-lib"})
}

func TestParsePackageJSON_Dedup(t *testing.T) {
	// Same name in multiple sections should only appear once.
	data := []byte(`{
  "dependencies": {"lodash": "^4.0.0"},
  "devDependencies": {"lodash": "^4.0.0"}
}`)
	deps, err := ParsePackageJSON(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertNames(t, deps, []string{"lodash"})
}
