package output

import (
	"fmt"
	"io"
	"sort"

	DepIsConfuse "github.com/Saher-M102/DepIsConfuse/internal"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBold   = "\033[1m"
)

// Summary is the outcome of printing a set of results, used by the caller
// to decide the process exit code.
type Summary struct {
	Total          int
	Claimed        int
	Unclaimed      int
	ScopeProtected int
	Skipped        int
	Errored        int
}

// PrintResults writes a human-readable report of the scan to w and
// returns a summary the caller can use to set the process exit code.
func PrintResults(w io.Writer, results []DepIsConfuse.Result) Summary {
	sorted := make([]DepIsConfuse.Result, len(results))
	copy(sorted, results)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Dependency.Registry != sorted[j].Dependency.Registry {
			return sorted[i].Dependency.Registry < sorted[j].Dependency.Registry
		}
		return sorted[i].Dependency.Name < sorted[j].Dependency.Name
	})

	var summary Summary
	var unclaimed, scopeProtected, skipped, errored []DepIsConfuse.Result

	for _, r := range sorted {
		summary.Total++
		switch r.Status {
		case DepIsConfuse.StatusError:
			summary.Errored++
			errored = append(errored, r)
		case DepIsConfuse.StatusClaimed:
			summary.Claimed++
		case DepIsConfuse.StatusScopeProtected:
			summary.ScopeProtected++
			scopeProtected = append(scopeProtected, r)
		case DepIsConfuse.StatusSkipped:
			summary.Skipped++
			skipped = append(skipped, r)
		default: // StatusUnclaimed
			summary.Unclaimed++
			unclaimed = append(unclaimed, r)
		}
	}

	fmt.Fprintf(w, "%sScanned %d dependenc%s%s\n\n", colorBold, summary.Total, plural(summary.Total), colorReset)

	if len(unclaimed) > 0 {
		fmt.Fprintf(w, "%s⚠ UNCLAIMED — potential dependency confusion risk:%s\n", colorRed+colorBold, colorReset)
		for _, r := range unclaimed {
			fmt.Fprintf(w, "  %s✗%s %s (%s) — not found on public registry, an attacker could claim this name\n",
				colorRed, colorReset, r.Dependency.Name, r.Dependency.Registry)
		}
		fmt.Fprintln(w)
	}

	if len(errored) > 0 {
		fmt.Fprintf(w, "%s⚠ ERRORS — could not verify these:%s\n", colorYellow+colorBold, colorReset)
		for _, r := range errored {
			fmt.Fprintf(w, "  %s?%s %s (%s) — %v\n",
				colorYellow, colorReset, r.Dependency.Name, r.Dependency.Registry, r.Err)
		}
		fmt.Fprintln(w)
	}

	if len(scopeProtected) > 0 {
		fmt.Fprintf(w, "%sℹ SCOPE-PROTECTED — name unclaimed, but the scope is registered so it can't be squatted:%s\n", colorBold, colorReset)
		for _, r := range scopeProtected {
			fmt.Fprintf(w, "  %s•%s %s (%s) — package name is free, but only members of scope %q can publish under it\n",
				colorGreen, colorReset, r.Dependency.Name, r.Dependency.Registry, registryScope(r.Dependency.Name))
		}
		fmt.Fprintln(w)
	}

	if len(skipped) > 0 {
		fmt.Fprintf(w, "%sℹ SKIPPED — matched a --safe pattern, not checked:%s\n", colorBold, colorReset)
		for _, r := range skipped {
			fmt.Fprintf(w, "  %s•%s %s (%s)\n", colorGreen, colorReset, r.Dependency.Name, r.Dependency.Registry)
		}
		fmt.Fprintln(w)
	}

	if len(unclaimed) == 0 && len(errored) == 0 {
		fmt.Fprintf(w, "%s✓ No dependency confusion risk found across %d dependenc%s.%s\n",
			colorGreen, summary.Total, plural(summary.Total), colorReset)
	} else if len(unclaimed) == 0 {
		fmt.Fprintf(w, "%s✓ No unclaimed dependencies found (some could not be fully verified — see errors above).%s\n", colorGreen, colorReset)
	}

	return summary
}

// registryScope extracts the "@scope" portion of a scoped npm name for
// display purposes; returns the whole name unchanged if unscoped.
func registryScope(name string) string {
	for i, c := range name {
		if c == '/' {
			return name[:i]
		}
	}
	return name
}

func plural(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}
