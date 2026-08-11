package parser

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"

	DepIsConfuse "github.com/Saher-M102/DepIsConfuse/internal"
)

// Matches the package name at the start of a requirements.txt line.
var requirementNameRe = regexp.MustCompile(`^\s*([A-Za-z0-9][A-Za-z0-9._-]*)`)

// ParseRequirementsTxt extracts unique package names from requirements.txt.
func ParseRequirementsTxt(data []byte) ([]DepIsConfuse.Dependency, error) {
	seen := make(map[string]struct{})
	scanner := bufio.NewScanner(bytes.NewReader(data))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
	// Skip pip directives and VCS URLs.
		if strings.HasPrefix(line, "-") || strings.Contains(line, "://") {
			continue
		}

		match := requirementNameRe.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		seen[match[1]] = struct{}{}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read requirements.txt: %w", err)
	}

	return toSortedPyPIDeps(seen), nil
}

// Matches TOML section headers such as "[project]"..
var pyprojectSectionRe = regexp.MustCompile(`^\[([^\]]+)\]\s*$`)

// Matches quoted strings used in dependency arrays.
var pyprojectStringRe = regexp.MustCompile(`"([^"]+)"|'([^']+)'`)

// pyprojectKeyRe extracts the key from a "key = value" TOML line.
var pyprojectKeyRe = regexp.MustCompile(`^([A-Za-z0-9_.-]+)\s*=`)

// ParsePyprojectToml extracts dependency names from the common
// PEP 621 and Poetry dependency sections in pyproject.toml.
func ParsePyprojectToml(data []byte) ([]DepIsConfuse.Dependency, error) {
	seen := make(map[string]struct{})
	scanner := bufio.NewScanner(bytes.NewReader(data))

	currentSection := ""
	inDependencyArray := false

	// Only the "dependencies" array is relevant in the main project section.
	isPEP621MainSection := func(section string) bool {
		return section == "project"
	}
	isPEP621OptionalSection := func(section string) bool {
		return strings.HasPrefix(section, "project.optional-dependencies") ||
			strings.HasPrefix(section, "dependency-groups")
	}
	isPoetryDepsSection := func(section string) bool {
		return strings.HasPrefix(section, "tool.poetry.dependencies") ||
			strings.HasPrefix(section, "tool.poetry.dev-dependencies") ||
			strings.HasPrefix(section, "tool.poetry.group.") && strings.HasSuffix(section, ".dependencies")
	}

	// Extract package names from quoted dependency strings.
	collectArrayStrings := func(line string) {
		for _, m := range pyprojectStringRe.FindAllStringSubmatch(line, -1) {
			raw := m[1]
			if raw == "" {
				raw = m[2]
			}
			if name := extractRequirementName(raw); name != "" {
				seen[name] = struct{}{}
			}
		}
	}

	for scanner.Scan() {
		rawLine := scanner.Text()
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if m := pyprojectSectionRe.FindStringSubmatch(line); m != nil {
			currentSection = m[1]
			inDependencyArray = false
			continue
		}

		switch {
		case isPEP621MainSection(currentSection):
		// Only scan the "dependencies" array in [project].
			if !inDependencyArray {
				if !(strings.HasPrefix(line, "dependencies") && strings.Contains(line, "=")) {
					continue
				}
				inDependencyArray = true
			}
			collectArrayStrings(line)
			if strings.Contains(line, "]") {
				inDependencyArray = false
			}

		case isPEP621OptionalSection(currentSection):
		// Each key here represents a dependency group such as dev or test.
			if !inDependencyArray {
				if !strings.Contains(line, "=") || !strings.Contains(line, "[") {
					continue
				}
				inDependencyArray = true
			}
			collectArrayStrings(line)
			if strings.Contains(line, "]") {
				inDependencyArray = false
			}

		case isPoetryDepsSection(currentSection):
			km := pyprojectKeyRe.FindStringSubmatch(line)
			if km == nil {
				continue
			}
			name := km[1]
			if strings.EqualFold(name, "python") {
				continue // the python version constraint isn't a package
			}
			seen[name] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read pyproject.toml: %w", err)
	}

	return toSortedPyPIDeps(seen), nil
}

// Extracts the package name from a PEP 508 requirement string.
func extractRequirementName(requirement string) string {
	match := requirementNameRe.FindStringSubmatch(requirement)
	if match == nil {
		return ""
	}
	return match[1]
}

func toSortedPyPIDeps(seen map[string]struct{}) []DepIsConfuse.Dependency {
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)

	deps := make([]DepIsConfuse.Dependency, 0, len(names))
	for _, name := range names {
		deps = append(deps, DepIsConfuse.Dependency{Name: name, Registry: DepIsConfuse.PyPI})
	}
	return deps
}
