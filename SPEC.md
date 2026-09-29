# Minesweeper — Specification

| | |
|---|---|
| **Status** | Draft v0.3 — 2026-09-29 |
| **Implementation** | Not started (see [§13 Milestones](#13-milestones)) |
| **Audience** | Whoever builds or maintains this project. This file is the source of truth: change the spec first, then the code. |
| **Changes from v0.2** | Dependencies are now allowed *if minimal* and a real window results. The terminal UI is replaced by a **native window** built on one library (Ebitengine), confined to a single package. The terminal design is preserved in git history (commit `659d62f`) as the zero-dependency fallback. §2, §3, §4, §6, §7, §9, §11 were rewritten. v0.1 (browser UI) was dropped earlier. |

Requirement keywords **MUST**, **SHOULD**, **MAY** follow RFC 2119. Every requirement has an ID (`C-1`, `G-4`, …) so tests, commits and reviews can cite it.

---

## 1. Overview

A clone of classic Windows Minesweeper that runs on **Windows and Linux** as a **native desktop window**, from a single downloaded file.

**Goals**

- Faithful classic gameplay and look: the three standard difficulties, first-click safety, flagging, chording, LED counters, smiley face, best times.
- One executable per OS. On Windows nothing else is needed; on Linux only the graphics libraries every desktop already has.
- A small, readable codebase with the rules and the entire UI (including how it looks) covered by automated tests that need no display.

**Non-goals (v1)**

- Browser or terminal UIs, multiplayer, online leaderboards, accounts, telemetry, update checks.
- Solver, hints, or no-guess board generation.
- Sound, saving an in-progress game, installers, code signing, localisation beyond English.

---

## 2. Hard constraints

| ID | Constraint |
|---|---|
| C-1 | The game **MUST** run as a native window on Windows 10+ (amd64, arm64) and Linux (amd64, arm64; X11 or Wayland-with-XWayland session). |
| C-2 | **Windows:** the end user **MUST NOT** need to install anything. **Linux:** the end user **MUST NOT** need to install anything beyond the X11 and OpenGL client libraries that ship with any desktop distribution ([§3](#3-technology-decision) lists them). No interpreter, VM, or framework on either OS. |
| C-3 | Third-party code is **restricted to one direct dependency, Ebitengine** (`github.com/hajimehoshi/ebiten/v2`), plus the modules it needs, under the policy in [§3.2](#32-dependency-policy). Everything else is the Go standard library. |
| C-4 | Building **MUST NOT** require a C compiler, C headers, or system development packages: release binaries are built with `CGO_ENABLED=0` and cross-compile from any OS. Only the Go toolchain is needed. |
| C-5 | The game **MUST NOT** open any network socket, send telemetry, or check for updates. |
| C-6 | The game **MUST NOT** depend on, launch, or render in a web browser. |

---

## 3. Technology decision

**Decision:** **Go** (see [B-1](#build-and-release-m0--m5) for the required version) with **Ebitengine v2** as the only windowing/input dependency. The game draws its own frames into a plain in-memory image (`*image.RGBA`, standard library) and Ebitengine only opens the window, delivers input, and shows that image.

### 3.1 What was verified before choosing this

Throwaway probe programs, run on the development machine (Linux + Xvfb X server), with Ebitengine v2.10.4:

- **Builds with `CGO_ENABLED=0` for all four targets** (`linux`/`windows` × `amd64`/`arm64`) from a Linux machine — no C compiler, no headers. Windows probes built as GUI-subsystem programs (`-H windowsgui`). Binary sizes were roughly **9–13 MB**.
- **Only five modules are compiled in:** `ebiten/v2`, `ebitengine/purego`, `ebitengine/hideconsole`, `golang.org/x/sys`, `golang.org/x/sync` — identical for the Linux and Windows builds. (A larger set appears in `go list -m all` but is not built into the binary.)
- **It works as a real window on Linux.** Under Xvfb the window opened and rendered; reading pixels back from the X server showed exactly the colours drawn; **injected X11 mouse events — left, middle and right button — and a key press arrived at the correct coordinates.** That is everything the game needs from input, including the middle button for chording.
- **Linux runtime needs:** the CGO-free binary is still dynamically linked to the system loader (**glibc**), and at runtime loads `libGL`, `libX11`, `libXcursor`, `libXrandr`, `libXinerama`, `libXi` and `libXxf86vm` (plus their own dependencies). These are present on desktop installs and absent on minimal/server installs.
- **No display → a clean error**, not a crash: `no window system is available …`, exit code 1.
- **Requires a very new Go:** Ebitengine v2.10.4 declares `go 1.25.0`.

**Not verified** (and therefore scheduled as risks, [§10](#10-non-functional-requirements-and-risks)): running on Windows (compiled only — no Windows or Wine here), Wayland-only systems, HiDPI scaling, idle CPU use, musl-based distros.

### 3.2 Dependency policy

| ID | Requirement |
|---|---|
| D-1 | `go.mod` **MUST** have exactly one direct `require`: Ebitengine, pinned to an exact version. `go.sum` is committed. |
| D-2 | The set of third-party modules *compiled into* any of the four targets **MUST** be a subset of an allowlist kept in `tools/checkdeps` (initially the five modules listed above). CI fails if an upgrade or change adds one. |
| D-3 | **Confinement:** only `internal/shell` and `cmd/minesweeper` may import Ebitengine, directly or otherwise. `engine`, `store` and `ui` compile and test without it. A test enforces this ([T-8](#11-testing-strategy)). |
| D-4 | Upgrading Ebitengine is a deliberate, single-purpose change: bump the pin, re-run `tools/checkdeps`, re-run the full acceptance checklist ([§12](#12-acceptance-checklist)) on both OSes. |
| D-5 | Because of D-3, replacing Ebitengine later means rewriting only `internal/shell` (target: a few hundred lines). |

### 3.3 Alternatives considered

| Option | Verdict | Reason |
|---|---|---|
| **Go + Ebitengine, own software renderer** | **Chosen** | One direct dependency; builds without cgo for all targets; real window verified on Linux; input covers all mouse buttons; confined to one package. |
| Go + terminal UI (v0.2 design) | Fallback | Zero dependencies, but not a window. Kept in git history (`659d62f`). |
| Go + hand-written Win32 and X11 windows | Runner-up | Zero dependencies, but two large platform backends, no Wayland-only support, Windows untestable from Linux. A pure-Go X11 client library did work as a test driver in the probe, but a full window was not attempted. Possible later behind D-5. |
| Fyne, Gio | Rejected | Not probed; per their documentation both need cgo and C headers on Linux, violating C-4, and are much larger. |
| Python + tkinter | Rejected | Needs an interpreter; `tkinter` is a separate package on many Linux distros. |
| Java / Swing, .NET | Rejected | Need a JRE / large runtime bundles; cross-platform GUI is third-party. |
| Rust / C / C++ with SDL, GTK, egui | Rejected | More dependencies, C toolchains, and system libraries than the chosen option. |
| Browser UI | Rejected | Violates C-6. |

---

## 4. Architecture

```
 cmd/minesweeper        flags, wiring, exit codes, error reporting
       │
       ▼
 internal/shell   ← the ONLY package that imports Ebitengine (D-3)
   opens the window · turns Ebitengine input into ui.Input · shows the frame
       │  ui.Input                       ▲  *image.RGBA frame + window size
       ▼                                 │
 internal/ui      pure Go, no display needed
   Model (state machine) · layout & hit-testing · pixel font & sprites
   · software renderer → *image.RGBA
       │ uses                    │ uses
       ▼                         ▼
 internal/engine  rules     internal/store  settings + best times (JSON file)
```

| ID | Requirement |
|---|---|
| A-1 | `internal/engine` **MUST** have no I/O, no `time` calls and no package-level randomness. The clock and RNG seed are injected, so every behaviour is testable and deterministic. |
| A-2 | `internal/ui` is a pure function of its inputs: `Model.Update(Input) → Effects`, then `Model.Frame() → (*image.RGBA, dirty)`. `Input` is a plain struct (mouse position and buttons, keys pressed, typed characters, focus, time). The UI **MUST NOT** import Ebitengine, the OS, or the disk (persistence is reached through an interface). |
| A-3 | All drawing is done by `internal/ui` into an `image.RGBA` at **logical (unscaled) resolution**. The shell scales it to the window. There are **no image, font or sound files**: sprites and the pixel font are defined in Go source. |
| A-4 | Package dependencies: `engine` and `store` import nothing from this repo; `ui` imports `engine` (and a `store` interface); `shell` imports `ui`; `cmd` imports `shell`, `ui`, `store`. |
| A-5 | The whole program is single-threaded from the UI's point of view: Ebitengine calls `Update` and `Draw` on one goroutine, and only they touch the `Model`. |

### Repository layout (target)

```
cmd/minesweeper/main.go     entry point
internal/engine/            rules, board, mine placement, flood fill, chord
internal/ui/                model, layout, menus/dialogs, pixel font, sprites, software renderer
internal/shell/             Ebitengine window, input mapping, presentation (the only ebiten importer)
internal/store/             settings + best-times persistence
tools/build/main.go         cross-platform build/release script (Go, not make/bash)
tools/checkdeps/main.go     dependency allowlist and confinement check
.github/workflows/          ci.yml, release.yml
SPEC.md  README.md  go.mod  go.sum
```

---

## 5. Game rules

### 5.1 Board and difficulty

| Preset | Width × Height | Mines |
|---|---|---|
| Beginner | 9 × 9 | 10 |
| Intermediate | 16 × 16 | 40 |
| Expert | 30 × 16 | 99 |
| Custom | 9–50 × 9–30 | 1 to `W×H − 9` |

| ID | Requirement |
|---|---|
| G-1 | The engine **MUST** reject configurations outside the table's bounds with a typed error. `mines ≤ W×H − 9` is what guarantees [G-3](#52-mine-placement) is always satisfiable. |
| G-2 | Each cell is in exactly one of: hidden, flagged, question-marked (only if enabled), revealed. Revealed cells carry the count (0–8) of adjacent mines (8-neighbourhood). |

### 5.2 Mine placement

| ID | Requirement |
|---|---|
| G-3 | Mines are placed **on the first reveal**, not at game start. The clicked cell **and its in-bounds neighbours** (the 3×3 block) **MUST** be mine-free, so the first click always opens a region (it is a `0`). |
| G-4 | Placement **MUST** yield exactly the configured number of mines, uniformly at random among allowed cells. |
| G-5 | Given the same seed, board size, mine count and first-click cell, the layout **MUST** be identical — across runs, OSes, and Go versions. Do not use stdlib helpers whose output may change between Go releases (e.g. `rand.Shuffle`, `IntN`); draw from a fixed `Source` (`math/rand/v2` PCG) and use an in-repo bounded-random routine. A **golden test** pins the layout for several seeds so an accidental change fails CI. |
| G-6 | Flags placed before the first reveal are kept and do not influence placement. |

> **Design note — deviation from Windows.** The original protects only the clicked cell. Protecting the 3×3 block is the common modern convention and avoids a forced guess on move one. Revisit only via a spec change.

### 5.3 Actions

| ID | Action | Behaviour |
|---|---|---|
| G-7 | **Reveal** (x, y) | Hidden or `?` cell: reveal it. Flagged or already-revealed cell: no-op (not an error). Revealing a mine loses the game. |
| G-8 | **Flood fill** | Revealing a `0` cell reveals all connected `0` cells and their bordering numbered cells. Flagged cells are never revealed by flood fill. **MUST** be iterative (explicit queue/stack) — no recursion. |
| G-9 | **Flag** (x, y) | Hidden → flag → hidden. With question marks enabled: hidden → flag → `?` → hidden. Revealed cells: no-op. Allowed before the first reveal. |
| G-10 | **Chord** (x, y) | On a revealed cell with count *n* > 0: if the number of adjacent flags **equals** *n*, reveal every adjacent hidden or `?` cell that is not flagged (with flood fill). Otherwise no-op. If a flag was wrong, this reveals a mine and loses; every mine revealed that way is marked as triggered. Chording a `0`, a hidden cell, or a cell whose flag count ≠ *n* is a no-op. |
| G-11 | Only **flags** count toward chords and the mine counter; `?` does not. |
| G-12 | Any action after the game has ended is rejected with a typed error; the state is unchanged. |

### 5.4 Win and lose

| ID | Requirement |
|---|---|
| G-13 | **Win** the moment every non-mine cell is revealed. Flags are irrelevant to the win condition. On win: timer stops, all unflagged mines are auto-flagged, mine counter shows 0. |
| G-14 | **Lose** on revealing a mine. On loss: timer stops; every unflagged mine is shown; each flag on a non-mine is shown crossed out; the mine(s) that triggered the loss are highlighted red. |

### 5.5 Counter and timer

| ID | Requirement |
|---|---|
| G-15 | Mine counter = `mines − flags placed`. It may go negative. Display is clamped to −99…999. |
| G-16 | The timer starts at the **first successful reveal**, is measured with a monotonic clock supplied to the engine ([A-1](#4-architecture)), and stops on win or loss. The display shows whole elapsed seconds, capped at 999 (the game itself continues past 999). |
| G-17 | Best times store milliseconds; only wins on the three presets are recorded ([P-3](#8-persistence)). |

---

## 6. User interface

A fixed-size window that recreates the classic Windows look, drawn entirely by `internal/ui`.

```
┌─ Minesweeper ──────────────────────── ─ □ ✕ ┐   ← OS title bar
│ Game   Help                                   │   ← in-window menu bar
├───────────────────────────────────────────────┤
│ ┌───────────────────────────────────────────┐ │
│ │  [010]                :)             [042]│ │   LED counter · face · LED timer
│ └───────────────────────────────────────────┘ │
│ ┌───────────────────────────────────────────┐ │
│ │ ▢▢▢▢▢▢▢▢▢▢ …  bevelled 16×16 cells         │ │
│ └───────────────────────────────────────────┘ │
└───────────────────────────────────────────────┘
```

### Look and layout

| ID | Requirement |
|---|---|
| U-1 | **Classic style:** grey (#C0C0C0) panels; white/dark-grey bevels; hidden cells raised, revealed cells flat with a thin grid line; header panel with a 3-digit red-on-black seven-segment **mine counter** (left), **face button** (centre), and **timer** (right). Cells are **16×16 logical pixels**. All metrics live in one `layout.go`. |
| U-2 | **Numbers 1–8** use the classic colours (1 blue #0000FF, 2 green #008000, 3 red #FF0000, 4 navy #000080, 5 maroon #800000, 6 teal #008080, 7 black, 8 grey #808080). Flags, mines, wrong-flags and question marks are distinct **shapes**, so colour is never the only signal. |
| U-3 | **Face:** smiling by default; "surprised" while a mouse button is held on the board; "dead" (X eyes) after a loss; "cool" (sunglasses) after a win. Clicking it starts a new game with the current board. |
| U-4 | **Text and sprites** come from a built-in pixel font (ASCII, defined in Go) and sprites defined as small bitmaps in Go source — no font, image, or asset files. |
| U-5 | **Themes:** `classic` (default) and `dark` (same layout, dark palette). `dark` **SHOULD** keep all text at WCAG AA contrast. |
| U-6 | **Scale:** the logical frame is shown at an integer scale of 1×–4× with nearest-neighbour filtering, so pixels stay crisp. Default `auto` = the largest of 1×–4× not exceeding `round(2 × display scale factor)` that still lets the Expert window fit on the monitor; Options and `--scale N` override it. The window resizes itself when the board or scale changes. |
| U-7 | **Window:** title "Minesweeper"; not user-resizable; closing it quits; centred on first show. |

### Mouse

| ID | Requirement |
|---|---|
| U-8 | While a button is held over the board, the cell under the pointer (or the 3×3 block for a chord gesture) is drawn pressed and the face is "surprised". Actions apply on **release**, on the cell under the pointer; releasing outside the board cancels. |
| U-9 | **Left** = reveal; **right** = flag; **middle**, or **left and right held together** = chord. |
| U-10 | **Click-to-chord** (`clickNumberChords`, default **on**): a plain left-click on a revealed number chords, so chording works on trackpads without a middle button. |
| U-11 | **Stuck-button safety:** if the window loses focus, or a release is never delivered, the gesture is cancelled and the display returns to normal. |

### Keyboard

| ID | Requirement |
|---|---|
| U-12 | Arrow keys move a focus cell (drawn with a dotted rectangle); `Home`/`End` = row start/end, `PgUp`/`PgDn` = top/bottom row. `Space`/`Enter` = reveal (on a revealed number: chord, per U-10); `F` = flag; `C` = chord; `F2` = new game; `F10` = open the menu bar (arrows + `Enter` to choose); `Esc` closes menus and dialogs. The whole game **MUST** be playable without a mouse. |

### Menus and dialogs

Menus and dialogs are drawn inside the window (the OS menu bar is not used), so they look identical on both platforms.

| ID | Requirement |
|---|---|
| U-13 | **Game menu:** New (`F2`) · Beginner · Intermediate · Expert · Custom… · *(separator)* · Marks `(?)` (checkbox) · Click number to chord (checkbox) · Theme › classic / dark · Scale › auto / 1× / 2× / 3× / 4× · *(separator)* · Best Times… · *(separator)* · Exit. The current difficulty is checked. **Help menu:** Controls… · About. |
| U-14 | **Custom…** has width, height and mines fields (`Tab` moves between them; digits and `Backspace` edit), with live validation against [G-1](#51-board-and-difficulty), and OK / Cancel. |
| U-15 | **Best Times:** top 5 per preset, with a Reset button that asks for confirmation. |
| U-16 | **End of game:** the face changes ([U-3](#look-and-layout)); on a top-5 win a name-entry dialog appears (printable ASCII, ≤ 20 characters, pre-filled with the last name; [P-4](#8-persistence)). The seed of the finished game **SHOULD** be shown in **About** or the end dialog so a board can be reproduced with `--seed`. |

### Accessibility (honest scope)

| ID | Requirement |
|---|---|
| U-17 | Fully keyboard-playable (U-12), scalable up to 4×, high-contrast-capable via the `dark` theme, and no reliance on colour alone (U-2). |
| U-18 | **Known limitation:** the game is a custom-drawn canvas, so it exposes no accessibility tree and **screen readers cannot read it**. This is a trade-off of the chosen approach and is stated in the README. |

---

## 7. Windowing shell

`internal/shell` is the only code that touches Ebitengine. It is deliberately thin.

| ID | Requirement |
|---|---|
| W-1 | **Window:** create it at the size the `Model` requests (logical size × scale), set the title, disable user resizing, and resize (`SetWindowSize`) whenever the `Model` reports a new size. Closing the window or `Effects.Quit` ends the program with exit 0. |
| W-2 | **Input mapping:** each `Update`, build one `ui.Input` from Ebitengine: pointer position in logical pixels, left/middle/right button state, keys pressed this tick (with auto-repeat for arrows and `Backspace`), typed characters, and window focus. No Ebitengine types leak out of the package. |
| W-3 | **Presentation:** when the `Model` reports the frame is dirty, upload the `*image.RGBA` to a texture and draw it scaled by an integer factor with nearest-neighbour filtering ([U-6](#look-and-layout)); otherwise draw nothing new. |
| W-4 | **Idle cost:** the tick rate is capped at 30 per second and an unchanged frame is not re-uploaded. Target: under 2 % of one core when idle — **to be measured** at M3/M5; if Ebitengine cannot skip drawing cleanly, lower the idle tick rate instead. |
| W-5 | **Startup failure** (no display, no GL, etc.): on Linux print one clear line to stderr and exit 1; on Windows, where the GUI-subsystem build has no console, show a native message box (`user32!MessageBoxW` through the standard `syscall` package) and exit 1. `--help` and `--version` output uses the same two paths. |
| W-6 | **`--smoke`:** open the window, run about 30 frames including one scripted click, exit 0; any initialisation failure exits non-zero. Used by CI on Linux under a virtual X server (`xvfb-run`). |
| W-7 | The timer runs on the engine's monotonic clock, so it is unaffected if the window stops updating while minimised or unfocused. |

---

## 8. Persistence

| ID | Requirement |
|---|---|
| P-1 | State lives in one file, `state.json`, in `<config dir>/minesweeper/`, where `<config dir>` is `os.UserConfigDir()` (Linux: `$XDG_CONFIG_HOME` or `~/.config`; Windows: `%AppData%`). `--data-dir` overrides it (`--data-dir .` gives a portable install). |
| P-2 | Schema v1: `{"version":1,"settings":{"questionMarks":false,"clickNumberChords":true,"theme":"classic","scale":0},"lastBoard":{…},"lastName":"…","bestTimes":{"beginner":[{"name":"…","ms":12340,"at":"<RFC 3339 UTC>"}],"intermediate":[…],"expert":[…]}}` (`scale` 0 = auto). |
| P-3 | Best times: top **5** per preset, ascending by `ms`; on a tie the earlier record ranks first. Custom boards are never recorded. |
| P-4 | A qualifying win is saved **immediately** under the last-used name (default `Anonymous`), so a crash cannot lose it; the name dialog then renames that record. Names are trimmed, ≤ 20 characters, printable ASCII only. |
| P-5 | Writes are atomic: write a temp file in the same directory, `fsync`, then `os.Rename` over the target (Go's Windows rename replaces existing files). Settings are written when changed and best times when earned, not only on exit. |
| P-6 | A corrupt or unknown-version file **MUST NOT** crash the game or be overwritten silently: rename it to `state.json.bad-<unix time>`, start with defaults, and show a notice in the window. The file is read with a 1 MiB cap. |
| P-7 | If the data directory cannot be created or written, the game **MUST** still run, without persistence, and says so in the window. |
| P-8 | The game writes nothing outside the data directory. |

---

## 9. Process lifecycle

### Command-line options

| Flag | Default | Meaning |
|---|---|---|
| `--data-dir PATH` | per-user config dir | Where `state.json` lives. |
| `--seed N` | random | Deterministic games: game *k* uses a seed derived from `N` and *k*. |
| `--scale N` | auto | Force scale 1–4 for this run (overrides the saved option). |
| `--smoke` | off | Self-test mode ([W-6](#7-windowing-shell)). |
| `--version`, `--help` | | Show and exit 0 ([W-5](#7-windowing-shell) for where it is shown). |

Exit codes: `0` normal, `1` runtime or startup error, `2` usage error.

| ID | Requirement |
|---|---|
| L-1 | The Windows build is a **GUI-subsystem** program (`-H windowsgui`): double-clicking the `.exe` opens only the game window, with no console. |
| L-2 | On Linux the binary is a normal executable (`chmod +x`, then run from a terminal or a launcher). A `.desktop` file and icon are out of scope for v1. |
| L-3 | One game exists per process. Two copies running at once share one `state.json`; the last writer wins. This is a documented limitation, not something to lock against. |

---

## 10. Non-functional requirements and risks

| ID | Requirement |
|---|---|
| N-1 | **Startup:** window visible within 1 s of launch on modest hardware. |
| N-2 | **Responsiveness:** click → repainted result under 50 ms on the largest board (50×30). Engine reveal with full flood fill under 5 ms. |
| N-3 | **Size:** each release binary ≤ 20 MB (the probe measured roughly 9–13 MB before game code). |
| N-4 | **Memory:** target under 100 MB resident (a budget to confirm at M5, not yet measured). |
| N-5 | **Supported systems:** Windows 10+ with a DirectX-capable graphics stack; Linux on a **glibc** distribution with X11 or XWayland, OpenGL, and the libraries listed in [§3.1](#31-what-was-verified-before-choosing-this). musl-based distros (e.g. Alpine) are unsupported. |
| N-6 | **Strings:** all user-visible text lives in one `strings` table so a later translation touches one place. |

### Risks

| ID | Risk | Mitigation |
|---|---|---|
| R-1 | **Nothing has been run on Windows.** The Windows build compiles, but window creation, DirectX use, mouse buttons and `-H windowsgui` behaviour are unverified. | M2 exists to prove the window + input + frame path on real Windows 10 and 11 *before* the game is built on it. If Ebitengine misbehaves there, D-5 limits the fix to `internal/shell`. |
| R-2 | **Ebitengine churn and a very new Go requirement** (`go 1.25.0` at v2.10.4). Contributors need a recent toolchain; APIs may change. | Exact pin (D-1); deliberate upgrades (D-4); `GOTOOLCHAIN=auto` fetches the toolchain; README states the version. |
| R-3 | **Linux runtime libraries and glibc.** A minimal install or musl distro will not start. | Clear startup error (W-5); README lists the packages; N-5 declares scope. |
| R-4 | **Wayland-only sessions** (no XWayland) are unsupported by this backend. | Documented; expected to work under normal GNOME/KDE Wayland via XWayland — verify in §12. |
| R-5 | **Idle CPU:** a game loop that redraws constantly wastes battery. | W-4 cap and measurement; fallback to a lower idle tick rate. |
| R-6 | **Single third-party dependency is still a supply-chain surface.** | D-1…D-4: exact pin, `go.sum`, allowlist check, reviewed upgrades. |
| R-7 | **No screen-reader support** (U-18). | Stated openly; keyboard play and scaling provided. |

---

## 11. Testing strategy

Because `ui` produces an in-memory image and takes plain input structs ([A-2](#4-architecture), [A-3](#4-architecture)), almost everything is testable with plain `go test` — no display, no GPU, no Ebitengine.

| ID | Area | Required tests |
|---|---|---|
| T-1 | Engine — first click | For thousands of seeds and every board size/corner/edge/centre click, including maximum density (`mines = W×H − 9`): first reveal never hits a mine and always produces a `0`. |
| T-2 | Engine — placement | Exact mine count; every cell's number equals a brute-force recount; determinism for equal seed + first click; golden layouts for several seeds ([G-5](#52-mine-placement)). |
| T-3 | Engine — flood fill | Matches a naive reference implementation on small boards; a sparse 50×30 board completes without deep recursion; flags block fill. |
| T-4 | Engine — actions and end states | Table-driven cases for reveal, flag cycle (with/without `?`), chord (exact, too few, too many flags, wrong flag → loss, on a `0`, on a hidden cell), post-game rejection; win detection, loss reveal, negative counter, timer start/stop/cap using a fake clock. |
| T-5 | UI model | Feed `Input` sequences to `Model.Update` and assert on state: a full win and loss, flag cycle, every mouse gesture (left, right, middle, left+right in both orders, release outside board = cancel, focus loss = cancel), click-to-chord on/off, keyboard-only play, menus (including `F10` navigation), Custom validation, name entry, hit-testing for every board size and scale. |
| T-6 | UI rendering | **Golden images:** render fixed scenarios (fresh Beginner board, mid-game, won, lost, each menu, each dialog, both themes, scales 1× and 2×) to PNG and compare with `testdata/*.png`; `go test -update` regenerates them for deliberate changes. Deterministic because rendering is pure software. Also unit tests for the LED digits and pixel font. |
| T-7 | Store | Temp dir: round trip; atomic write leaves no partial file; corrupt file → `.bad-*` backup and defaults; unwritable dir → runs without persistence; top-5 ordering and tie-break. |
| T-8 | Dependency rules | (a) `go list -deps` per target shows compiled third-party modules ⊆ the allowlist ([D-2](#32-dependency-policy)); (b) parsing imports shows only `shell` and `cmd` import Ebitengine ([D-3](#32-dependency-policy)); (c) `go.mod` has exactly one direct `require` ([D-1](#32-dependency-policy)). Implemented in `tools/checkdeps`, run by CI. |
| T-9 | Window smoke (Linux CI) | `xvfb-run ./minesweeper --smoke` exits 0 ([W-6](#7-windowing-shell)) — proves window, GL and input plumbing start on a clean runner. |
| T-10 | Windows | `GOOS=windows go vet ./...` and cross-build in CI; behaviour verified by the manual checklist on real machines (R-1). If a Windows CI runner can open a window, add `--smoke` there as best-effort. |

**Gates (CI):** `gofmt -l .` empty · `go vet ./...` for `linux` and `windows` · `go test ./...` on Linux **and** Windows · `go test -race ./...` on Linux (needs a C compiler in CI, which is fine — only *building the product* must not) · `tools/checkdeps` · engine statement coverage ≥ 95 %, `ui` ≥ 85 %, overall ≥ 80 %.

---

## 12. Acceptance checklist

Run manually on a **clean** Windows 10, Windows 11, and a mainstream Linux desktop in both an X11 session and a Wayland session (no Go or dev tools installed), using only the downloaded release file.

- [ ] Runs from the downloaded file; on Windows double-click opens only the game window (no console).
- [ ] Beginner, Intermediate, Expert and a Custom board have the right size and mine count; the window resizes correctly when switching.
- [ ] Looks crisp (no blur) at scales 1×–4× and on a HiDPI display; `auto` picks a sensible size.
- [ ] First click never loses and always opens a region, including in a corner.
- [ ] Left, right, middle, and left+right behave as in [U-8, U-9](#mouse); click-to-chord works on a trackpad; releasing outside the board cancels; alt-tabbing mid-press does not leave a stuck button.
- [ ] Playable entirely from the keyboard ([U-12](#keyboard)) including the menu via `F10`.
- [ ] Win → cool face, auto-flagged mines, counter 0, name prompt on a top-5 time; loss → dead face, mines / crossed flags / red triggered mine.
- [ ] Timer starts on first reveal and stops at the end.
- [ ] Best times and settings persist across restarts; the file is where [P-1](#8-persistence) says and nothing else is written elsewhere.
- [ ] Corrupt `state.json` → game still starts, backup file created, notice shown.
- [ ] Both themes readable; every menu and dialog usable at 1× and 4×.
- [ ] Idle CPU under 2 % of a core with the window open and untouched ([W-4](#7-windowing-shell)).
- [ ] On Linux with no display (`env -u DISPLAY`) and, if reproducible, with missing libraries: one clear error line, exit 1. On Windows, `--help` shows a message box.
- [ ] With the network disabled, everything works; no sockets opened by the game (C-5).

---

## 13. Milestones

Update the checkboxes here (and the README status line) as work lands. The order is **risk-first**: the unverified Windows window comes before the game is built on it.

| | Milestone | Exit criteria |
|---|---|---|
| [ ] | **M0 Scaffold** | `go.mod` (one require, exact pin), `cmd/minesweeper` prints `--version`, `tools/checkdeps`, CI green on Ubuntu + Windows, cross-builds for all four targets. |
| [ ] | **M1 Engine** | §5 fully implemented; T-1 … T-4 pass; coverage ≥ 95 %. |
| [ ] | **M2 Window proof** | `internal/shell` + a minimal `ui` that renders a grid to an RGBA image and reports clicks/keys. T-8, T-9 pass; **verified by hand on real Windows 10 and 11 and on Linux X11 and Wayland — R-1 closed, or the shell is adapted.** |
| [ ] | **M3 Playable game** | Pixel font, sprites, LED digits, classic rendering, core play (reveal, flag, chord, win/lose, counters, face), keyboard and mouse gestures, scale; T-5, T-6 pass; idle CPU measured (W-4). |
| [ ] | **M4 Full UI + persistence** | Menus and all dialogs (U-13 … U-16), themes, Custom, Best Times, §8; T-7 passes. |
| [ ] | **M5 Release** | `go run ./tools/build` produces all four binaries + `SHA256SUMS`; release workflow on tags; NFRs measured; §12 checklist passed; README screenshots. |

### Build and release (M0 / M5)

| ID | Requirement |
|---|---|
| B-1 | The required Go version is whatever Ebitengine's `go` directive demands (**1.25.0** at the pinned v2.10.4); `go.mod` records it and the README states it. |
| B-2 | Build tooling is written in **Go** (`go run ./tools/build`), not `make` or shell scripts, so the identical command works on Windows and Linux without extra installs. |
| B-3 | Release builds use `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=<v>"`, plus `-H windowsgui` for Windows targets; version from `git describe --tags --always --dirty` (fallback `dev`). |
| B-4 | Targets: `linux/amd64`, `linux/arm64`, `windows/amd64`, `windows/arm64`. |
| B-5 | Output names: `minesweeper-<version>-<os>-<arch>[.exe]` in `dist/`, plus `SHA256SUMS`. |
| B-6 | Pushing a tag `vX.Y.Z` (semver) triggers a workflow that builds all targets and attaches the files to a GitHub Release. |
| B-7 | Binaries are **unsigned**; Windows SmartScreen will warn on first run. The README documents this. Code signing is out of scope. |

---

## 14. Out of scope and future work

Not in v1; candidates in rough priority order:

1. **Dropping the dependency:** hand-written Win32 and X11 backends behind the `internal/shell` boundary (D-5), giving a zero-dependency build again.
2. **Terminal front-end** as a zero-dependency fallback, revived from the v0.2 design in git history.
3. `.desktop` entry, application icon, and installers.
4. **No-guess board generator** and a hint/solver built on the engine.
5. Save and resume an in-progress game.
6. Sound effects (opt-in).
7. Localisation (see N-6).
8. Replay/export of a game from its seed and move list.
