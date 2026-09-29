// Command minesweeper is a classic Minesweeper for Windows and Linux.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/wortheydaniel/minesweeper/internal/shell"
	"github.com/wortheydaniel/minesweeper/internal/store"
	"github.com/wortheydaniel/minesweeper/internal/ui"
)

// version is set at build time: -ldflags "-X main.version=v1.2.3".
var version = "dev"

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	fs := flag.NewFlagSet("minesweeper", flag.ContinueOnError)
	var usage bytes.Buffer
	fs.SetOutput(&usage)
	dataDir := fs.String("data-dir", "", "directory for settings and best times (default: user config dir)")
	seed := fs.Uint64("seed", 0, "fixed seed for reproducible boards")
	scale := fs.Int("scale", 0, "window scale 1-4 (default: automatic)")
	smoke := fs.Bool("smoke", false, "self-test: open the window, run a few frames, exit")
	showVersion := fs.Bool("version", false, "print the version and exit")
	fs.SetOutput(io.Discard)

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fs.SetOutput(&usage)
			fmt.Fprintln(&usage, "Usage: minesweeper [options]")
			fs.PrintDefaults()
			shell.Message("Minesweeper", usage.String(), false)
			return 0
		}
		shell.Message("Minesweeper", err.Error()+"\nTry --help.", true)
		return 2
	}
	if *showVersion {
		shell.Message("Minesweeper", "minesweeper "+version, false)
		return 0
	}
	if *scale < 0 || *scale > 4 {
		shell.Message("Minesweeper", "--scale must be between 1 and 4", true)
		return 2
	}

	opt := ui.Options{Store: store.Open(*dataDir), Scale: *scale, Version: version}
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "seed" {
			opt.Seed = seed
		}
	})
	if err := shell.Run(shell.Config{Model: ui.New(opt), Smoke: *smoke}); err != nil {
		shell.Message("Minesweeper", "Could not run the game:\n"+err.Error(), true)
		return 1
	}
	return 0
}
