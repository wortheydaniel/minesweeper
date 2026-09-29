// Command checkdeps enforces the dependency rules from SPEC section 3.2:
//
//   - go.mod has exactly one direct requirement (Ebitengine);
//   - the third-party modules compiled into each release target are a subset
//     of the allowlist below;
//   - only internal/shell and cmd/minesweeper import Ebitengine.
//
// Run it with: go run ./tools/checkdeps
package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

const ebiten = "github.com/hajimehoshi/ebiten/v2"

// allowed are the third-party modules that may be compiled into the game.
// Verified for Ebitengine v2.10.4; recheck when upgrading (SPEC D-4).
var allowed = map[string]bool{
	ebiten:                              true,
	"github.com/ebitengine/purego":      true,
	"github.com/ebitengine/hideconsole": true,
	"golang.org/x/sys":                  true,
	"golang.org/x/sync":                 true,
}

var targets = []string{"linux/amd64", "linux/arm64", "windows/amd64", "windows/arm64"}

func main() {
	var problems []string
	fail := func(f string, a ...any) { problems = append(problems, fmt.Sprintf(f, a...)) }

	// D-1: one direct requirement.
	out, err := exec.Command("go", "list", "-m", "-f", "{{if not .Indirect}}{{.Path}}{{end}}", "all").Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "go list:", err)
		os.Exit(2)
	}
	var direct []string
	for _, l := range strings.Fields(string(out)) {
		if !strings.HasPrefix(l, "github.com/wortheydaniel/minesweeper") {
			direct = append(direct, l)
		}
	}
	if len(direct) != 1 || direct[0] != ebiten {
		fail("go.mod direct requirements must be exactly [%s], got %v", ebiten, direct)
	}

	// D-2: compiled-in modules per target.
	for _, t := range targets {
		parts := strings.Split(t, "/")
		cmd := exec.Command("go", "list", "-deps", "-f", "{{with .Module}}{{.Path}}{{end}}", "./cmd/minesweeper")
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+parts[0], "GOARCH="+parts[1])
		out, err := cmd.Output()
		if err != nil {
			fail("%s: go list failed: %v", t, err)
			continue
		}
		seen := map[string]bool{}
		for _, m := range strings.Fields(string(out)) {
			if !strings.HasPrefix(m, "github.com/wortheydaniel/minesweeper") {
				seen[m] = true
			}
		}
		var extra []string
		for m := range seen {
			if !allowed[m] {
				extra = append(extra, m)
			}
		}
		sort.Strings(extra)
		if len(extra) > 0 {
			fail("%s: modules not on the allowlist: %v", t, extra)
		}
	}

	// D-3: Ebitengine is confined to internal/shell and cmd.
	out, err = exec.Command("go", "list", "-f", "{{.ImportPath}} {{join .Imports \" \"}}", "./...").Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "go list:", err)
		os.Exit(2)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Fields(line)
		pkg := f[0]
		ok := strings.HasSuffix(pkg, "/internal/shell") || strings.HasSuffix(pkg, "/cmd/minesweeper")
		for _, imp := range f[1:] {
			if strings.HasPrefix(imp, ebiten) && !ok {
				fail("%s imports %s; only internal/shell may", pkg, imp)
			}
		}
	}

	if len(problems) > 0 {
		for _, p := range problems {
			fmt.Fprintln(os.Stderr, "FAIL:", p)
		}
		os.Exit(1)
	}
	fmt.Println("checkdeps: ok")
}
