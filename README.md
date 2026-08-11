# DepIsConfuse

DepIsConfuse is a dependency confusion scanner for npm and PyPI. It checks whether the dependencies of a project are actually claimed on their public registry — an unclaimed name is a real risk, since anyone could publish a malicious package under that exact name and, depending on how the project resolves packages, it could end up getting pulled in instead of the intended internal package.

## Features

- Scans local manifest files: `package.json`, `requirements.txt`, `pyproject.toml`
- Scans manifest files directly from a URL (GitHub blob links are converted to raw content automatically)
- Checks a single package name on demand
- Covers npm and PyPI, with the registry auto-detected from the manifest filename
- Understands npm scoped packages (`@org/name`) correctly, including scope-level protection — an unclaimed name under a scope you already own isn't a real risk
- Lets you assert known-safe namespaces manually with `-s/--safe` instead of relying on automatic detection
- Concurrent registry checks, CI-friendly exit codes
- Single static binary, no external dependencies, no API key required

## Usage

```
DepIsConfuse -h
```

This will display help for the tool.

```
DepIsConfuse — dependency confusion scanner for npm and PyPI

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
```

### Examples

```
DepIsConfuse -f package.json
DepIsConfuse --file requirements.txt
DepIsConfuse -u https://github.com/org/repo/blob/main/package.json
DepIsConfuse -d internal-auth-lib -n
DepIsConfuse -d company-utils -n -p
DepIsConfuse -f package.json -s '@mycompany/*,internal-*'
```

### Scoped npm packages

An unclaimed scoped package name (`@org/name`) is not automatically a risk. npm scopes are owned by an org or user, and only members of that scope can publish under it — so if `@org` is registered to someone, an outside attacker cannot claim `@org/anything`, even if that exact package name isn't published yet.

DepIsConfuse checks this automatically and reports such cases as `SCOPE-PROTECTED` rather than `UNCLAIMED`. This relies on npmjs.com's website, since there's no authenticated-free registry API for checking scope ownership — treat it as best-effort. If you'd rather not depend on it, assert ownership yourself with `-s/--safe`.

### Exit codes

| Code | Meaning |
|------|---------|
| 0    | No unclaimed dependencies found |
| 1    | At least one unclaimed dependency found (vulnerable) |
| 2    | Usage error, or the scan could not run (bad input, network failure) |

These are meant to be used directly in CI pipelines.

## Installation

DepIsConfuse requires go1.22 to install successfully. Run the following command to install the latest version:

```
go install -v github.com/Saher-M102/DepIsConfuse@latest
```

Alternatively, build from source:

```
git clone https://github.com/Saher-M102/DepIsConfuse.git
cd DepIsConfuse
go build -o DepIsConfuse .
```

No external Go dependencies are required — the tool is built entirely on the standard library.

## How it works

For each dependency, DepIsConfuse queries the relevant public registry to check whether the package name is claimed:

- **npm**: `GET https://registry.npmjs.org/<name>`
- **PyPI**: `GET https://pypi.org/pypi/<name>/json`

A `404` means the name is free — anyone could publish under it. A `200` means it's already taken, which is what you want for every internal/private dependency your project pulls in.

## License

MIT
