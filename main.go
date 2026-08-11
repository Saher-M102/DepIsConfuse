package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

		DepIsConfuse "github.com/Saher-M102/DepIsConfuse/internal"
	"github.com/Saher-M102/DepIsConfuse/internal/fetch"
	"github.com/Saher-M102/DepIsConfuse/internal/output"
	"github.com/Saher-M102/DepIsConfuse/internal/parser"
	"github.com/Saher-M102/DepIsConfuse/internal/registry"
)

const version = "0.1.0"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

/*

 Exit codes are kept separate so the tool can be used easily in scripts and CI.
    0 = no unclaimed dependencies
    1 = one or more unclaimed dependencies
     2 = the scan could not run

*/

const (
	exitOK           = 0
	exitVulnerable   = 1
	exitUsageOrError = 2
)

func run(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("DepIsConfuse", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		file        string
		urlInput    string
		depName     string
		useNPM      bool
		usePyPI     bool
		timeoutSec  int
		concurrency int
		showVersion bool
		safeRaw     string
	)

/*
 Register both short and long forms of the flags.
  The standard flag package doesn't support aliases directly.
*/
	fs.StringVar(&file, "f", "", "")
	fs.StringVar(&file, "file", "", "Path to a local dependency manifest (package.json, requirements.txt, pyproject.toml)")
	fs.StringVar(&urlInput, "u", "", "")
	fs.StringVar(&urlInput, "url", "", "URL to a remote dependency manifest (e.g. a GitHub file URL)")
	fs.StringVar(&depName, "d", "", "")
	fs.StringVar(&depName, "dependency", "", "A single dependency name to check (requires -n and/or -p)")
	fs.BoolVar(&useNPM, "n", false, "")
	fs.BoolVar(&useNPM, "npm", false, "Check against the public npm registry")
	fs.BoolVar(&usePyPI, "p", false, "")
	fs.BoolVar(&usePyPI, "pypi", false, "Check against the public PyPI registry")
	fs.IntVar(&timeoutSec, "timeout", 15, "HTTP timeout in seconds for each registry request")
	fs.IntVar(&concurrency, "c", 10, "")
	fs.IntVar(&concurrency, "concurrency", 10, "Number of concurrent registry checks")
	fs.BoolVar(&showVersion, "version", false, "Print the version and exit")
	fs.StringVar(&safeRaw, "s", "", "")
	fs.StringVar(&safeRaw, "safe", "", "Comma-separated list of known-safe namespaces (supports * wildcards), e.g. '@mycompany/*'")

	fs.Usage = func() { printUsage(stderr) }

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return exitOK
		}
		return exitUsageOrError
	}

	if showVersion {
		fmt.Fprintf(stdout, "DepIsConfuse version %s\n", version)
		return exitOK
	}

	deps, err := resolveDependencies(resolveInput{
		file:    strings.TrimSpace(file),
		url:     strings.TrimSpace(urlInput),
		depName: strings.TrimSpace(depName),
		useNPM:  useNPM,
		usePyPI: usePyPI,
	})
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n\n", err)
		printUsage(stderr)
		return exitUsageOrError
	}

	if len(deps) == 0 {
		fmt.Fprintln(stderr, "Error: no dependencies found to check")
		return exitUsageOrError
	}

	safePatterns := parseSafePatterns(safeRaw)

	client := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
	results := registry.CheckAll(client, deps, concurrency, safePatterns)

	summary := output.PrintResults(stdout, results)
	if summary.Unclaimed > 0 {
		return exitVulnerable
	}
	return exitOK
}

/*
	Parse the --safe value into individual patterns.
	Empty values and extra spaces are ignored.

*/
func parseSafePatterns(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	patterns := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			patterns = append(patterns, p)
		}
	}
	return patterns
}

/*
Holds the input options used to choose where dependencies come from and which registries should be checked.

*/
type resolveInput struct {
	file    string
	url     string
	depName string
	useNPM  bool
	usePyPI bool
}

/*
 Validate the input flags and build the list of dependencies to check.
 The input can come from a local file, a URL, or a single dependency name.

*/
func resolveDependencies(in resolveInput) ([]DepIsConfuse.Dependency, error) {
	sourceCount := 0
	if in.file != "" {
		sourceCount++
	}
	if in.url != "" {
		sourceCount++
	}
	if in.depName != "" {
		sourceCount++
	}

	switch sourceCount {
	case 0:
		return nil, fmt.Errorf("no input specified — use one of -f/--file, -u/--url, or -d/--dependency")
	default:
		if sourceCount > 1 {
			return nil, fmt.Errorf("only one of -f/--file, -u/--url, or -d/--dependency may be used at a time")
		}
	}

	switch {
	case in.depName != "":
		return resolveSingleDependency(in.depName, in.useNPM, in.usePyPI)

	case in.file != "":
		if in.useNPM || in.usePyPI {
			return nil, fmt.Errorf("-n/--npm and -p/--pypi are not used with -f/--file — the registry is auto-detected from the manifest filename")
		}
		data, err := os.ReadFile(in.file)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %q: %w", in.file, err)
		}
		return parseByFilename(filepath.Base(in.file), data)

	case in.url != "":
		if in.useNPM || in.usePyPI {
			return nil, fmt.Errorf("-n/--npm and -p/--pypi are not used with -u/--url — the registry is auto-detected from the manifest filename")
		}
		client := &http.Client{Timeout: 15 * time.Second}
		data, err := fetch.FetchURL(client, in.url)
		if err != nil {
			return nil, err
		}
		return parseByFilename(fetch.FileNameFromURL(in.url), data)
	}

	
	return nil, fmt.Errorf("internal error: no input branch matched")
}

// resolveSingleDependency builds the dependency list for -d, tagging the
// name against every explicitly requested registry.
func resolveSingleDependency(name string, useNPM, usePyPI bool) ([]DepIsConfuse.Dependency, error) {
	if !useNPM && !usePyPI {
		return nil, fmt.Errorf("-d/--dependency requires at least one registry flag: -n/--npm and/or -p/--pypi")
	}
	var deps []DepIsConfuse.Dependency
	if useNPM {
		deps = append(deps, DepIsConfuse.Dependency{Name: name, Registry: DepIsConfuse.NPM})
	}
	if usePyPI {
		deps = append(deps, DepIsConfuse.Dependency{Name: name, Registry: DepIsConfuse.PyPI})
	}
	return deps, nil
}

//Choose the parser based on the manifest filename.
// Unknown filenames are rejected instead of guessing the file type.
func parseByFilename(filename string, data []byte) ([]DepIsConfuse.Dependency, error) {
	switch filename {
	case "package.json":
		return parser.ParsePackageJSON(data)
	case "requirements.txt":
		return parser.ParseRequirementsTxt(data)
	case "pyproject.toml":
		return parser.ParsePyprojectToml(data)
	default:
		return nil, fmt.Errorf(
			"unrecognized manifest filename %q — DepIsConfuse only auto-detects standard names "+
				"(package.json, requirements.txt, pyproject.toml); rename the file or point -f/-u at one of those",
			filename,
		)
	}
}

func printUsage(w *os.File) {
	fmt.Fprint(w, `DepIsConfuse — dependency confusion scanner for npm and PyPI

Checks whether your project's dependencies are claimed on the public
registry. An unclaimed name is a dependency confusion risk: an attacker
could publish a malicious package under that exact name.

USAGE:
  DepIsConfuse -f <path>              Scan a local manifest file
  DepIsConfuse -u <url>                Scan a manifest file from a URL (e.g. GitHub)
  DepIsConfuse -d <name> -n|-p         Check a single package name

INPUT (choose exactly one):
  -f, --file <path>        Local manifest file. Registry is auto-detected
                            from the filename:
                              package.json       -> npm
                              requirements.txt   -> pypi
                              pyproject.toml      -> pypi
  -u, --url <url>          Remote manifest file URL (GitHub blob URLs are
                            automatically converted to raw content URLs).
                            Same filename-based auto-detection as -f.
  -d, --dependency <name>  A single package name to check. Requires at
                            least one of -n/--npm or -p/--pypi.

REGISTRY (only used together with -d):
  -n, --npm                Check the name against the public npm registry
  -p, --pypi                Check the name against the public PyPI registry

OPTIONS:
  -s, --safe <patterns>    Comma-separated list of known-safe namespaces,
                            supports * wildcards (e.g. '@mycompany/*').
                            Matching dependencies are skipped entirely.
  -c, --concurrency <n>    Number of concurrent registry checks (default 10)
      --timeout <seconds>  HTTP timeout per registry request (default 15)
      --version             Print version and exit
  -h, --help                 Show this help

ABOUT SCOPED NPM PACKAGES (@org/name):
  An unclaimed scoped package name isn't automatically a risk. npm scopes
  are owned by an org/user, and only members can publish under a scope
  they own — so if the scope (e.g. "@org") is registered to someone, an
  outside attacker cannot claim "@org/anything", even if that exact name
  isn't published yet. DepIsConfuse checks this automatically and reports
  such cases as SCOPE-PROTECTED rather than UNCLAIMED. This check relies
  on npmjs.com's website (there's no authenticated-free registry API for
  it), so it's best-effort — use -s/--safe to assert it yourself if you'd
  rather not depend on that.

EXAMPLES:
  DepIsConfuse -f package.json
  DepIsConfuse --file requirements.txt
  DepIsConfuse -u https://github.com/org/repo/blob/main/package.json
  DepIsConfuse -d internal-auth-lib -n
  DepIsConfuse -d company-utils -n -p
  DepIsConfuse -f package.json -s '@mycompany/*,internal-*'

EXIT CODES:
  0   no unclaimed dependencies found (safe)
  1   at least one unclaimed dependency found (vulnerable)
  2   usage error or the scan could not run (bad input, network failure)
`)
}
