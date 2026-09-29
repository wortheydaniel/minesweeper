# Minesweeper

Classic Minesweeper for **Windows and Linux**, as a native desktop window with the classic look. One downloaded file — no installer, no browser, no runtime.

> **Status: playable, Linux-tested.** The game is implemented as specified in [`SPEC.md`](SPEC.md) and all automated tests pass. It has been run for real on Linux under a virtual X server: the window opens, the self-test's scripted click plays, and a screenshot of the running window was checked. Physical mouse and keyboard input has not been tried with the finished game (an earlier probe showed Ebitengine delivers all three mouse buttons and keys correctly on X11). **The Windows build compiles but has never been run** — no Windows machine was available — so treat Windows as untested until you try it. CI is written but hasn't run yet. See [SPEC §13](SPEC.md#13-milestones) for what's left.

## Just want to play?

No installing, no building. Download one file and run it:

| Your computer | Download | Then |
|---|---|---|
| **Windows** 10 or 11 | [`downloads/minesweeper-windows.exe`](downloads/minesweeper-windows.exe) | Double-click it. If Windows says *"Windows protected your PC"*, click **More info → Run anyway** (the file isn't code-signed). |
| **Linux** desktop | [`downloads/minesweeper-linux`](downloads/minesweeper-linux) | Right-click → Properties → *Allow executing*, then double-click. Or in a terminal: `chmod +x minesweeper-linux && ./minesweeper-linux` |

On GitHub, open the file and click the **download** button (the arrow icon near the top right of the file view).

Your best times and settings are saved automatically. The Windows file has never been run on a real Windows machine yet — if it doesn't open, please tell me what you see.

For ARM computers, or to build it yourself, see [Building from source](#building-from-source) below.

## Requirements

**To play**

| OS | What you need |
|---|---|
| **Windows** 10 or newer (amd64 or arm64) | Nothing. |
| **Linux** (amd64 or arm64) | A **glibc** desktop distribution (Debian, Ubuntu, Fedora, Arch, …) running an X11 session, or Wayland with XWayland (the default on GNOME and KDE), with OpenGL. |

On a normal Linux desktop the required libraries are already installed. On a minimal install, add them — on Debian/Ubuntu:

```
sudo apt install libgl1 libx11-6 libxcursor1 libxrandr2 libxinerama1 libxi6 libxxf86vm1
```

Alpine and other musl-based distributions are not supported. There is no Python, Java, .NET, Node, or browser involved, and the game never uses the network.

**To build from source**

[Go](https://go.dev/dl/) **1.25 or newer** (Go can download the right toolchain itself). No C compiler and no system development packages are needed.

## Running the release files in detail

The files in [`downloads/`](downloads) are built from this repository (64-bit Intel/AMD only):

**Windows** — double-click `minesweeper-<version>-windows-amd64.exe`. Windows SmartScreen may warn about an unrecognised app because the binary is unsigned: choose *More info → Run anyway*.

**Linux**
```
chmod +x minesweeper-<version>-linux-amd64
./minesweeper-<version>-linux-amd64
```

## How to play

Reveal every square that isn't a mine. A number tells you how many of the 8 neighbouring squares are mines. The first click is always safe and always opens an area.

| Action | Mouse | Keyboard |
|---|---|---|
| Move focus | — | Arrow keys (`Home`/`End`, `PgUp`/`PgDn` jump) |
| Reveal | Left-click | `Space` / `Enter` |
| Flag (and `?` if enabled) | Right-click | `F` |
| Chord — open all neighbours of a satisfied number | Middle-click, left+right together, or left-click the number (on by default) | `C` |
| New game | Click the face | `F2` |
| Menu | Click **Game** / **Help** | `F10` |
| Cancel a menu or dialog | — | `Esc` |

Difficulties: **Beginner** 9×9/10 mines, **Intermediate** 16×16/40, **Expert** 30×16/99, or **Custom**. Best times are kept for the top 5 on each preset. The **Game** menu also has question marks, click-to-chord, the theme (classic or dark; each click switches), and the window scale (auto, 1×–4×; each click cycles). **Help → About** shows the seed of the last finished game.

## Command-line options

| Option | Default | Purpose |
|---|---|---|
| `--scale N` | auto | Window scale 1–4 for this run. |
| `--data-dir PATH` | per-user config dir | Where settings and best times are stored. `--data-dir .` makes a portable install. |
| `--seed N` | random | Reproducible boards, useful for bug reports and tests. |
| `--smoke` | off | Self-test: open the window, click once, run a few frames, exit 0 on success (used by CI). |
| `--version`, `--help` | | Show and exit. On Windows these appear in a message box, because the program has no console. |

## Where your data goes

One file, `state.json` (settings and best times):

| OS | Default location |
|---|---|
| Linux | `~/.config/minesweeper/state.json` (or `$XDG_CONFIG_HOME/minesweeper/`) |
| Windows | `%AppData%\minesweeper\state.json` |

Delete it to reset everything. Nothing else is written anywhere.

## Building from source

```
git clone https://github.com/wortheydaniel/minesweeper
cd minesweeper

go run ./tools/build        # all four targets + SHA256SUMS into dist/
go build ./cmd/minesweeper  # just your own OS, for quick iteration
go run ./cmd/minesweeper    # build and run
```

The build script is written in Go (not `make` or a shell script) so the same commands work in PowerShell, cmd, and bash. Release builds use `CGO_ENABLED=0`, so a Windows `.exe` builds fine from Linux and vice versa.

## Testing

```
go test ./...
go run ./tools/checkdeps    # dependency rules
```

Almost nothing needs a display: `internal/engine` holds the rules, and `internal/ui` draws the whole game into an in-memory image, so tests feed it input and compare the result with golden PNGs in `internal/ui/testdata/` (regenerate deliberately with `go test ./internal/ui -update`, then look at the images). On Linux CI a smoke test opens the real window under a virtual X server: `xvfb-run ./minesweeper --smoke`. Behaviour on real Windows can only be checked by hand — use the checklist in [SPEC §12](SPEC.md#12-acceptance-checklist).

## Project layout

```
cmd/minesweeper/     entry point: flags, wiring, error reporting
internal/engine/     game rules — pure logic, no I/O, no clock, no global RNG
internal/ui/         model, layout, menus, pixel font, sprites, software renderer — no display needed
internal/shell/      the ONLY package that imports Ebitengine: window, input, presenting frames
internal/store/      settings and best times (atomic JSON file)
tools/build/         cross-platform build and release script
tools/checkdeps/     dependency allowlist and confinement check
SPEC.md              the specification (source of truth)
```

## Maintaining this project

**Spec first.** Behaviour and requirements are defined in [`SPEC.md`](SPEC.md). To change how the game behaves, edit the spec in the same commit as the code and cite requirement IDs (`G-10`, `D-3`, …) in commit messages and tests.

**Rules of the road** (each is enforced by CI or by review):

- **One dependency: Ebitengine.** `go.mod` has exactly one direct `require`, pinned exactly; `tools/checkdeps` fails CI if any other third-party module gets compiled in (SPEC D-1, D-2). Need something else? Write the small piece of code, or change the spec first.
- **Ebitengine stays in `internal/shell`.** Nothing else may import it, so the dependency can be replaced by rewriting one small package (SPEC D-3, D-5).
- **The UI draws itself.** `internal/ui` renders to an `image.RGBA` with no display, fonts, or image files; sprites and the pixel font are Go source (SPEC A-2, A-3). Visual changes show up as golden-image diffs — review them, then regenerate.
- **The engine stays pure.** `internal/engine` doesn't read the clock, touch the disk, or use package-level randomness; time and seed are injected (SPEC A-1).
- **Seeds stay stable.** Changing mine placement changes every reproducible board. A golden test will fail if you do; only update it deliberately (SPEC G-5).
- **Upgrading Ebitengine is its own change.** Bump the pin, run `checkdeps`, and re-run the acceptance checklist on Windows *and* Linux (SPEC D-4). It also sets the minimum Go version (SPEC B-1).
- **Windows can't be tested from Linux.** Changes to `internal/shell` need a manual run on real Windows (SPEC R-1, §12).

**Common tasks**

| Task | Where |
|---|---|
| Add a difficulty preset | Engine preset table (SPEC §5.1) → Game menu in `internal/ui` |
| Add a setting | SPEC §8 schema → `internal/store` → Game menu; give it a default so old `state.json` files still load |
| Change the `state.json` format | Bump `version`, keep a reader for the old one (SPEC P-6 covers unknown versions) |
| Change colours, sprites, or layout | `internal/ui` (`layout.go`, sprite bitmaps, theme tables); regenerate golden images and review them |
| Add a menu item or dialog | SPEC U-13…U-16 → `internal/ui`; add a golden image and a keyboard-navigation test |
| Raise the minimum Go version | It follows Ebitengine's `go` directive; update `go.mod`, the Requirements above, SPEC B-1, and the CI matrix |
| Cut a release | Update SPEC §13, tag `vX.Y.Z` and push the tag; the release workflow builds and uploads the binaries (SPEC B-6) |

**Known limitations**

- **No screen-reader support.** The game is a custom-drawn window with no accessibility tree (SPEC U-18). It is fully keyboard-playable and scalable.
- Linux needs a glibc system with X11/XWayland and OpenGL; Wayland *without* XWayland, and musl distros, are unsupported.
- Binaries are unsigned, so Windows SmartScreen warns on first run.
- Two copies running at once share one `state.json`; the last writer wins.
- macOS is not a target.

**Troubleshooting**

| Symptom | Cause / fix |
|---|---|
| Linux: `no window system is available` | No display: run it inside a desktop session (or over `ssh -X`). |
| Linux: `cannot open shared object file` / fails to start | Missing graphics libraries — install the packages listed under Requirements. |
| Linux: blank or black window in a VM or remote desktop | No GPU acceleration. Try software rendering: `LIBGL_ALWAYS_SOFTWARE=1 ./minesweeper-…`. |
| Window is tiny on a high-resolution screen | Use **Game → Scale**, or `--scale 3`. |
| Windows: nothing appears | Startup errors appear in a message box. This build has never been run on Windows, so please report what you see. |
| Best times vanished | A corrupt `state.json` is renamed to `state.json.bad-<time>` and defaults are used; look for that file beside it. |

## License

Not yet chosen. Add a `LICENSE` file before publishing binaries or accepting outside contributions.
