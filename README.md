# Minesweeper

Classic Minesweeper for **Windows and Linux**. One file, no installer, no runtime, no dependencies — download it, run it, and it opens in your browser.

> **Status: specification stage — no code yet.**
> The design is finished and lives in [`SPEC.md`](SPEC.md); implementation is tracked by the milestones in [SPEC §13](SPEC.md#13-milestones). Everything below describes the **intended** interface. The commands will work once milestone M5 lands; until then, treat this README as the contract the code must meet.
>
> Current milestone: **M0 — not started**

## Requirements

| To play | To build from source |
|---|---|
| Windows 10+ or Linux (amd64 or arm64) | [Go](https://go.dev/dl/) 1.22 or newer — nothing else |
| A web browser (Chrome, Edge, Firefox, Safari) | |

That's the complete list. There is no Python, Java, .NET, Node, npm, or system library to install, and the game never touches the internet.

## Quick start

Download the file for your system from the Releases page, then:

**Windows**
```
minesweeper-<version>-windows-amd64.exe
```
Double-click it, or run it from a terminal. A console window appears and your browser opens to the game. Windows SmartScreen may warn about an unrecognised app because the binary is unsigned: choose *More info → Run anyway*.

**Linux**
```
chmod +x minesweeper-<version>-linux-amd64
./minesweeper-<version>-linux-amd64
```

To quit: use **Quit** in the game menu, press `Ctrl+C` in the terminal, or close the console window. Closing the browser tab alone does **not** stop the program.

If no browser opens (for example on a headless machine), copy the printed `http://127.0.0.1:<port>/` address into a browser. Over SSH, forward the port first: `ssh -L 8080:127.0.0.1:8080 host` with `--port 8080`.

## How to play

Reveal every square that isn't a mine. A number tells you how many of the 8 neighbouring squares are mines.

| Action | Mouse | Keyboard |
|---|---|---|
| Reveal | Left-click | `Space` / `Enter` |
| Flag (and `?` if enabled) | Right-click | `F` |
| Chord — open all neighbours of a satisfied number | Middle-click, left+right together, or left-click the number (Options → on by default) | `C` |
| Move | — | Arrow keys |
| New game | Click the smiley | `N` or `F2` |

The first click is always safe and always opens an area. Difficulties: **Beginner** 9×9/10 mines, **Intermediate** 16×16/40, **Expert** 30×16/99, or **Custom**. Best times are kept for the top 5 on each preset.

## Command-line options

| Option | Default | Purpose |
|---|---|---|
| `--port N` | random free port | Port to listen on (always on `127.0.0.1` only). |
| `--no-browser` | off | Don't open a browser; just print the URL. |
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

Rules live entirely in `internal/engine` and are covered by unit and property tests (first-click safety, flood fill, chording, determinism, win/loss). The browser UI deliberately contains no rules, so it has no JS test suite; it's covered by the manual checklist in [SPEC §12](SPEC.md#12-acceptance-checklist).

## Project layout

```
cmd/minesweeper/     entry point: flags, signals, opens the browser
internal/engine/     game rules — pure logic, no I/O, no clock, no global RNG
internal/server/     loopback HTTP server + JSON API + security checks
internal/store/      settings and best times (atomic JSON file)
web/                 embedded UI: static/index.html, style.css, app.js
tools/build/         cross-platform build and release script
SPEC.md              the specification (source of truth)
```

*(Directories appear as their milestones land.)*

## Maintaining this project

**Spec first.** Behaviour, API, and requirements are defined in [`SPEC.md`](SPEC.md). To change how the game behaves, edit the spec in the same commit as the code and cite requirement IDs (`G-10`, `S-4`, …) in commit messages and tests.

**Rules of the road** (each is enforced by CI or by review):

- **No third-party dependencies, ever.** `go.mod` must have no `require` lines. If something seems to need a library, write the small piece of code instead. This is what keeps the project buildable with only Go and runnable with nothing at all (SPEC C-2, C-3).
- **The engine stays pure.** `internal/engine` doesn't read the clock, touch the disk, or use package-level randomness; time and seed are injected (SPEC A-2).
- **The JS never decides anything.** No game rules in `app.js`; if it needs a fact, add it to the API view (SPEC A-3).
- **The server never leaks mines.** While a game is running, the board sent to the browser must not contain mine positions. A test guards this (SPEC A-6); don't weaken it.
- **Seeds stay stable.** Changing mine placement changes every reproducible board. A golden test will fail if you do; only update it deliberately (SPEC G-5).
- **The API contract is in SPEC §7.** Change the spec, the handler, and `app.js` together.

**Common tasks**

| Task | Where |
|---|---|
| Add a difficulty preset | Engine preset table (SPEC §5.1) → API `preset` values (§7) → menu in `web/static/index.html` |
| Add a setting | SPEC §7 *Settings* → `internal/store` → Options dialog; give it a default so old `state.json` files still load |
| Change the `state.json` format | Bump `version`, keep a reader for the old one (SPEC P-6 covers unknown versions) |
| Raise the minimum Go version | `go.mod`, this README's Requirements table, SPEC N-5, and the CI matrix |
| Cut a release | Update SPEC §13, tag `vX.Y.Z` and push the tag; the release workflow builds and uploads the binaries (SPEC B-5) |

**Known limitations**

- A browser is required (a native window can't be done with zero dependencies — see SPEC §3).
- Binaries are unsigned, so Windows SmartScreen warns on first run.
- Two copies running at once share one `state.json`; the last writer wins.
- macOS should build and run but is not tested or supported.

**Troubleshooting**

| Symptom | Cause / fix |
|---|---|
| Browser doesn't open | No `xdg-open` (Linux) or no default browser. Copy the printed URL into a browser. |
| `address already in use` | You passed `--port` for a port that's taken. Omit `--port` to pick a free one. |
| Page loads blank on Windows | A wrong `.js` type in the registry can break embedded scripts; the server sets content types itself (SPEC S-6), so a blank page is a bug — report it. |
| Best times vanished | A corrupt `state.json` is renamed to `state.json.bad-<time>` and defaults are used; look for that file beside it. |
| Can't reach the game from another computer | By design: it listens on `127.0.0.1` only (SPEC S-1). Use SSH port forwarding. |

## License

Not yet chosen. Add a `LICENSE` file before publishing binaries or accepting outside contributions.
