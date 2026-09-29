// Command build cross-compiles release binaries for every supported target
// into dist/ and writes SHA256SUMS. It is Go, not make or shell, so the same
// command works on Windows and Linux:
//
//	go run ./tools/build
package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var targets = []struct{ goos, goarch string }{
	{"linux", "amd64"}, {"linux", "arm64"},
	{"windows", "amd64"}, {"windows", "arm64"},
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "build:", err)
		os.Exit(1)
	}
}

func version() string {
	if out, err := exec.Command("git", "describe", "--tags", "--always", "--dirty").Output(); err == nil {
		return strings.TrimSpace(string(out))
	}
	return "dev"
}

func run() error {
	v := version()
	if err := os.MkdirAll("dist", 0o755); err != nil {
		return err
	}
	var sums strings.Builder
	for _, t := range targets {
		name := fmt.Sprintf("minesweeper-%s-%s-%s", v, t.goos, t.goarch)
		ldflags := "-s -w -X main.version=" + v
		if t.goos == "windows" {
			name += ".exe"
			ldflags += " -H windowsgui" // no console window
		}
		out := filepath.Join("dist", name)
		cmd := exec.Command("go", "build", "-trimpath", "-ldflags", ldflags, "-o", out, "./cmd/minesweeper")
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+t.goos, "GOARCH="+t.goarch)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		fmt.Println("building", name)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		data, err := os.ReadFile(out)
		if err != nil {
			return err
		}
		fmt.Fprintf(&sums, "%x  %s\n", sha256.Sum256(data), name)
	}
	return os.WriteFile(filepath.Join("dist", "SHA256SUMS"), []byte(sums.String()), 0o644)
}
