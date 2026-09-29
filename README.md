# Minesweeper

Classic Minesweeper for **Windows and Linux**, played in your terminal with the mouse or keyboard. One file, no installer, no runtime, no dependencies — and no browser.

> **Status: specification stage — no code yet.**
> The design is finished and lives in [`SPEC.md`](SPEC.md); implementation is tracked by the milestones in [SPEC §13](SPEC.md#13-milestones). Everything below describes the **intended** interface. The commands will work once milestone M5 lands; until then, treat this README as the contract the code must meet.
>
> Current milestone: **M0 — not started**

## Requirements

| To play | To build from source |
|---|---|
| Windows 10 (version 1809 or newer) or Linux, on amd64 or arm64 | [Go](https://go.dev/dl/) 1.22 or newer — nothing else |
| A terminal (the Windows console, Windows Terminal, or any Linux terminal emulator) | |

That's the complete list. There is no Python, Java, .NET, Node, or system library to install, and the game never uses the network.

A terminal of **at least 80×24** is recommended; Expert needs 63×23. For the mouse you need a terminal that reports it (Windows Terminal, GNOME Terminal, Konsole, xterm, kitty, alacritty and most others do). Without a mouse the whole game is playable from the keyboard.

## Quick start

Download the file for your system from the Releases page, then:

**Windows** — double-click `minesweeper-<version>-windows-amd64.exe` (a console window opens with the game), or run it from a terminal. Windows SmartScreen may warn about an unrecognised app because the binary is unsigned: choose *More info → Run anyway*.

**Linux** — run it from a terminal:
```
chmod +x minesweeper-<version>-linux-amd64
./minesweeper-<version>-linux-amd64
```

Press `Q` to quit (or `Ctrl+C` to quit immediately).

## How to play

Reveal every square that isn't a mine. A number tells you how many of the 8 neighbouring squares are mines.

| Action | Mouse | Keyboard |
|---|---|---|
| Move | — | Arrow keys (`Home`/`End`, `PgUp`/`PgDn` jump) |
| Reveal | Left-click | `Space` / `Enter` |
| Flag (and `?` if enabled) | Right-click | `F` |
| Chord — open all neighbours of a satisfied number | Middle-click, left+right together, or left-click the number (on by default) | `C` |
| New game | Click the face `:)` | `N` or `F2` |
| Difficulty / Best times / Options / Help | Click the menu bar | `D` / `B` / `O` / `?` |
| Quit | Menu bar | `Q` or `Ctrl+C` |

The first click is always safe and always opens an area. Difficulties: **Beginner** 9×9/10 mines, **Intermediate** 16×16/40, **Expert** 30×16/99, or **Custom** (as large as your terminal allows). Best times are kept for the top 5 on each preset.

## Command-line options

| Option | Default | Purpose |
|---|---|---|
| `--glyphs auto\|ascii\|unicode` | `auto` | Symbol set for cells. Use `ascii` if you see boxes or question marks instead of symbols. |
| `--no-color` | off | Monochrome mode (also enabled by the `NO_COLOR` environment variable). |
| `--data-dir PATH` | per-user config dir | Where settings and best times are stored. `--data-dir .` makes a portable install. |
| `--seed N` | random | Reproducible boards, useful for bug reports and tests. |
| `--version`, `--help` | | Print and exit. |

## Where your data goes

One file, `state.json` (settings and best times):

| OS | Default location |
|---|---|
| Linux | `~/.config/minesweeper/state.json` (or `$XDG_CONFIG_HOME/minesweeper/`) |
| Windows | `%AppData%\minesweeper\state.json` |

Delete it to reset everything. Nothing else is written anywhere.

## Building from source

Requires only Go 1.22+.

```
git clone https://github.com/wortheydaniel/minesweeper
cd minesweeper

go run ./tools/build        # all four targets + SHA256SUMS into dist/
go build ./cmd/minesweeper  # just your own OS, for quick iteration
go run ./cmd/minesweeper    # build and run
```

The build script is written in Go (not `make` or a shell script) so the same commands work in PowerShell, cmd, and bash. Release builds are static (`CGO_ENABLED=0`), so a binary built on Linux for Windows just works.

## Testing

```
go test ./...
```

Almost everything is testable without a terminal: `internal/engine` holds the rules, and `internal/tui` is a pipeline of pure steps (input bytes → events → model → screen grid → diff), so tests feed events in and assert on the resulting screen text. The input decoder also has a fuzz target (`go test -fuzz=FuzzDecode ./internal/tui`). On Linux, an integration test runs the real binary inside a pseudo-terminal and checks the terminal is restored afterwards. The Windows console layer can only be checked on a Windows machine — use the checklist in [SPEC §12](SPEC.md#12-acceptance-checklist).

## Project layout

```
cmd/minesweeper/     entry point: flags, wiring, exit codes
internal/engine/     game rules — pure logic, no I/O, no clock, no global RNG
internal/tui/        events, input decoder, model, view, renderer, themes — no OS calls
internal/term/       the ONLY OS-specific code: raw mode, size, restore (term_linux.go, term_windows.go)
internal/store/      settings and best times (atomic JSON file)
tools/build/         cross-platform build and release script
SPEC.md              the specification (source of truth)
```

*(Directories appear as their milestones land.)*

## Maintaining this project

**Spec first.** Behaviour and requirements are defined in [`SPEC.md`](SPEC.md). To change how the game behaves, edit the spec in the same commit as the code and cite requirement IDs (`G-10`, `TL-3`, …) in commit messages and tests.

**Rules of the road** (each is enforced by CI or by review):

- **No third-party dependencies, ever.** `go.mod` must have no `require` lines. If something seems to need a library, write the small piece of code instead. This keeps the project buildable with only Go and runnable with nothing at all (SPEC C-2, C-3).
- **No network, no browser.** The game opens no sockets and launches nothing (SPEC C-5, C-6).
- **The engine stays pure.** `internal/engine` doesn't read the clock, touch the disk, or use package-level randomness; time and seed are injected (SPEC A-1).
- **OS-specific code stays in `internal/term`.** Everything else must compile and pass tests on every platform. Never call `syscall` from `tui` (SPEC A-3, A-5).
- **The terminal must always be restored.** Every exit path — quit, Ctrl+C, `SIGTERM`, panic — must undo raw mode, mouse reporting, and the alternate screen. A test checks this (SPEC TL-3, T-10); don't add an exit path that bypasses it.
- **Seeds stay stable.** Changing mine placement changes every reproducible board. A golden test will fail if you do; only update it deliberately (SPEC G-5).
- **Windows can't be tested from Linux.** Changes to `internal/term/term_windows.go` need a manual run on real Windows (SPEC R-1, §12).

**Common tasks**

| Task | Where |
|---|---|
| Add a difficulty preset | Engine preset table (SPEC §5.1) → Difficulty dialog in `internal/tui` |
| Add a setting | SPEC §8 schema → `internal/store` → Options dialog; give it a default so old `state.json` files still load |
| Change the `state.json` format | Bump `version`, keep a reader for the old one (SPEC P-6 covers unknown versions) |
| Change symbols or colours | The glyph table and the palette in `internal/tui/theme.go` (SPEC U-5, U-6); check both glyph sets on both OSes |
| Support a new key or mouse sequence | Decoder in `internal/tui` + a row in its test table (SPEC TL-4); keep the fuzz target green |
| Raise the minimum Go version | `go.mod`, this README's Requirements table, SPEC N-5, and the CI matrix |
| Cut a release | Update SPEC §13, tag `vX.Y.Z` and push the tag; the release workflow builds and uploads the binaries (SPEC B-5) |

**Known limitations**

- Needs a terminal; there is no graphical window (see SPEC §3 for why, and §14 for the option to add one).
- Boards larger than the terminal can't be scrolled; enlarge the window.
- Binaries are unsigned, so Windows SmartScreen warns on first run.
- Two copies running at once share one `state.json`; the last writer wins.
- macOS is not supported in v1 (it builds but exits with "unsupported platform").
- Linux: start it from a terminal; double-clicking in a file manager isn't supported.

**Troubleshooting**

| Symptom | Cause / fix |
|---|---|
| "not a terminal" / exit code 2 | Output is piped or redirected, or `TERM` is unset/`dumb`. Run it directly in a terminal window. |
| Boxes or `?` instead of symbols | The terminal font lacks the Unicode glyphs. Run with `--glyphs ascii`. |
| Colours hard to read | Some terminal themes remap the basic colours. Try `--no-color`, or another theme. |
| Mouse does nothing | The terminal isn't reporting the mouse: `tmux` needs `set -g mouse on`; the Linux text console has no mouse. Use the keyboard. |
| "Terminal too small" | Enlarge the window to the size shown (Expert needs 63×23). |
| Shell looks broken after a crash (no echo, garbage on click) | Run `reset` (Linux) or close and reopen the window. If it happens without a crash, that's a bug — the game must always restore the terminal. |
| Best times vanished | A corrupt `state.json` is renamed to `state.json.bad-<time>` and defaults are used; look for that file beside it. |

## License

Not yet chosen. Add a `LICENSE` file before publishing binaries or accepting outside contributions.
