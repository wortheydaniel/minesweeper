# Minesweeper — Specification

| | |
|---|---|
| **Status** | Draft v0.2 — 2026-09-29 |
| **Implementation** | Not started (see [§13 Milestones](#13-milestones)) |
| **Audience** | Whoever builds or maintains this project. This file is the source of truth: change the spec first, then the code. |
| **Changes from v0.1** | The browser-based UI was dropped (requirement: the game must not depend on or run in a web browser). It is now a **terminal application**; §3, §4, §6, §7 and §9 were rewritten and the HTTP API and all networking were removed. |

Requirement keywords **MUST**, **SHOULD**, **MAY** follow RFC 2119. Every requirement has an ID (`C-1`, `G-4`, …) so tests, commits and reviews can cite it.

---

## 1. Overview

A clone of classic Windows Minesweeper that runs on **Windows and Linux** from a **single downloaded file with nothing else to install**, in the player's terminal, with full mouse support.

**Goals**

- Faithful classic gameplay: the three standard difficulties, first-click safety, flagging, chording, timer, best times.
- One self-contained executable per OS. Download, run, play.
- Small, readable codebase that one person can maintain, with the game rules and the UI logic fully unit-tested.

**Non-goals (v1)**

- A graphical window (see [§3](#3-technology-decision) and [§14](#14-out-of-scope-and-future-work)), a browser UI, multiplayer, online leaderboards, accounts, telemetry, update checks.
- Solver, hints, or no-guess board generation.
- Saving an in-progress game across restarts, installers, code signing, localisation beyond English.

---

## 2. Hard constraints

These come straight from the brief and override every other preference in this document.

| ID | Constraint |
|---|---|
| C-1 | The game **MUST** run on Windows 10 version 1809 or newer (amd64, arm64) and Linux (amd64, arm64). |
| C-2 | The end user **MUST NOT** need to install anything: no interpreter, VM, framework, shared library, or installer. The only prerequisite is a terminal, which both operating systems already provide. |
| C-3 | The project **MUST** use no third-party code. `go.mod` has **zero** `require` lines; the standard library only. Building requires only the Go toolchain. |
| C-4 | Release binaries **MUST** be statically linked (`CGO_ENABLED=0`) so they do not depend on the user's glibc or any system library. |
| C-5 | The game **MUST NOT** open any network socket (listening or outgoing), send telemetry, or check for updates. It is a purely local program. |
| C-6 | The game **MUST NOT** depend on, launch, or render in a web browser. |

---

## 3. Technology decision

**Decision:** **Go** (minimum 1.22, standard library only) producing one static binary per OS, with a **terminal UI** (colour, keyboard and full mouse support).

### Why a terminal UI?

With no browser allowed (C-6) and no third-party code (C-3), the only display surface left that exists on every Windows and Linux machine is the terminal. A native window is technically possible without libraries, but only by hand-writing two separate window-system backends (Win32 through `syscall`, and the X11 wire protocol over a Unix socket, which also leaves Wayland-only systems unsupported). That is substantially more platform code than the terminal layer, and the Windows half cannot be tested from a Linux development machine. It is recorded as a possible future front-end ([§14](#14-out-of-scope-and-future-work)); the rules engine is I/O-free ([A-1](#4-architecture)) so adding it later would not touch the rules.

### Why Go?

- **Cross-compiles from any machine with no cgo:** `GOOS=windows go build` just works, so a contributor on Linux can produce the Windows `.exe`.
- **Static, dependency-free, small output.**
- **Everything needed is in the standard library** except a terminal package — which Go does not have, so a small `internal/term` package makes the raw system calls itself (see below).
- Strong typing, native fuzzing and `go test` make the rules and the input decoder easy to keep correct.

### What was verified before choosing this

A throwaway probe program (stdlib only) that switches the terminal to raw mode, enables alternate screen and SGR mouse reporting, reads the window size and reads input:

- **Built for all four targets** (`linux`/`windows` × `amd64`/`arm64`) with `CGO_ENABLED=0` from a Linux machine; `go vet` clean for Linux and Windows. Sizes were **1.4–1.5 MB** each.
- **Linux run for real inside a pseudo-terminal:** raw mode entered; window size read correctly; SGR mouse escape sequences (left and right button presses) arrived intact; **Ctrl+C arrived as an ordinary key byte** (`0x03`) rather than killing the process; alternate screen and mouse mode were switched on and off; **the terminal's original settings were fully restored** on a normal quit, on Ctrl+C-as-key, and on `SIGTERM`; and stdin that is not a terminal was refused with exit code 2.
- **Windows was compiled and vetted but not executed** — there is no Windows or Wine in the development environment. The Windows path (virtual-terminal input/output through `kernel32` calls) is the project's biggest unverified risk and is scheduled first ([R-1](#10-non-functional-requirements-and-risks), [§13 M2](#13-milestones)).

### Alternatives considered

| Option | Verdict | Reason |
|---|---|---|
| Browser UI (local web server or HTML file) | Rejected | Violates C-6. Was the v0.1 design. |
| Python + tkinter | Rejected | Needs an interpreter (not on Windows by default); `tkinter` is a separate package on many Linux distros. Violates C-2. |
| Java / Swing | Rejected | Needs a JRE. Violates C-2. |
| .NET | Rejected | WinForms is Windows-only; cross-platform GUI needs Avalonia (third-party); self-contained bundles are large. |
| Rust / C / C++ with a GUI library | Rejected | GUI libs (egui, SDL, GTK) are third-party, and on Linux need X11/Wayland/GL at runtime. Violates C-3/C-4. |
| Go + Fyne / Ebitengine | Rejected | Third-party; Linux builds need cgo and X11/GL development headers. |
| Go + hand-written Win32 and X11 windows | Runner-up (deferred) | Real windows with no libraries, but two large untested-from-here backends and no Wayland-only support. Possible later as a second front-end. |
| **Go + terminal UI** | **Chosen** | Zero dependencies at build and run time, small platform layer, testable, works over SSH. |

---

## 4. Architecture

```
 cmd/minesweeper            flags, wiring, exit codes
       │
       ▼
 internal/tui  ──uses──►  internal/engine   pure game rules
 events · decoder         internal/store    settings + best times (JSON file)
 model · view · renderer
       │  bytes / size
       ▼
 internal/term   raw mode, terminal size, VT setup, restore
 term_linux.go · term_windows.go · term_other.go (stub)
       │
   the user's terminal (Windows console / Windows Terminal / any ANSI terminal)
```

| ID | Requirement |
|---|---|
| A-1 | `internal/engine` **MUST** have no I/O, no `time` calls and no package-level randomness. The clock and RNG seed are injected, so every behaviour is testable and deterministic. |
| A-2 | The UI is a pipeline of pure steps: input bytes → `Decode` → **Events** (key, mouse, resize, tick) → `Model.Update(event)` → `Model.View()` → **Screen** (a grid of cells with glyph, colours, attributes) → `Diff(prev, next)` → output bytes. Only the outermost loop touches the terminal, so everything else is testable with no terminal at all. |
| A-3 | **All OS-specific code lives in `internal/term`**, one file per OS behind build tags, plus a stub for other platforms that returns an "unsupported platform" error so `go vet ./...` and cross-builds still succeed. |
| A-4 | One event loop owns the model and the screen. A reader goroutine (input), a ticker (250 ms: timer refresh and resize polling) and the signal handler only *send events* to it, so there are no data races and no locks in the UI. |
| A-5 | Dependencies between packages: `engine`, `store` and `term` import nothing from this repo; `tui` imports `engine` and `store`; `cmd` imports `tui` and `term`. `tui` never imports `term`. |

### Repository layout (target)

```
cmd/minesweeper/main.go     entry point
internal/engine/            rules, board, mine placement, flood fill, chord
internal/tui/               events, input decoder, model, view, renderer, glyph/colour themes
internal/term/              raw mode, size, VT setup, restore (per-OS files)
internal/store/             settings + best-times persistence
tools/build/main.go         cross-platform build/release script (Go, not make/bash)
.github/workflows/          ci.yml, release.yml
SPEC.md  README.md  go.mod
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
| G-14 | **Lose** on revealing a mine. On loss: timer stops; every unflagged mine is shown; each flag on a non-mine is shown as a wrong flag; the mine(s) that triggered the loss are highlighted. |

### 5.5 Counter and timer

| ID | Requirement |
|---|---|
| G-15 | Mine counter = `mines − flags placed`. It may go negative. Display is clamped to −99…999. |
| G-16 | The timer starts at the **first successful reveal**, is measured with a monotonic clock supplied to the engine ([A-1](#4-architecture)), and stops on win or loss. The display shows whole elapsed seconds, capped at 999 (the game itself continues past 999). |
| G-17 | Best times store milliseconds; only wins on the three presets are recorded ([P-3](#8-persistence)). |

---

## 6. User interface

A full-screen terminal UI in the alternate screen buffer, using **two terminal columns per cell** so the classic 30×16 Expert board fits a default 80×24 terminal.

```
 [N]ew  [D]ifficulty  [B]est times  [O]ptions  [?]Help  [Q]uit
 ┌─────────────────────────────────────────────────────────┐
 │  010                        :)                     042  │   mines · face · time
 └─────────────────────────────────────────────────────────┘
 ┌─────────────────────────────────────────────────────────┐
 │  ▒ ▒ ▒ ▒ 1   1 ► ▒ ▒ ▒ ▒ …                              │
 │  ▒ ▒ ▒ ▒ 1   1 2 ▒ ▒ ▒ ▒ …                              │
 └─────────────────────────────────────────────────────────┘
  Row 3, col 5: hidden                          (status line)
```

### Layout and rendering

| ID | Requirement |
|---|---|
| U-1 | Screen, top to bottom: menu bar (1 row); header box (3 rows) with the mine counter left, face centre, timer right; board frame (`H + 2` rows); status line (1 row). Required terminal size is **(2W + 3) columns × (H + 7) rows** — Beginner 21×16, Intermediate 35×23, Expert 63×23. |
| U-2 | **Too small:** if the terminal is smaller than U-1 requires, show only "Terminal too small — need C×R, have c×r. Enlarge the window, or press Q to quit" and resume automatically when it is large enough. The Custom dialog states the largest board that fits the current terminal and rejects larger ones. There is no scrolling viewport in v1. |
| U-3 | **Face** (plain ASCII, so width is never ambiguous): `:)` ready/playing, `:O` while a mouse button is held over the board, `X(` after a loss, `B)` after a win. Clicking it starts a new game with the current board settings. |
| U-4 | **Counters** are 3 digits, bold red on black (an "LED" look), zero-padded, `-` sign when negative. |
| U-5 | **Cell glyphs** come from one of two sets, chosen by [U-10](#6-user-interface): see the table below. Every cell state has a distinct glyph, so **colour is never the only signal**. |
| U-6 | **Colour:** 16-colour SGR only in v1 (no 256-colour/truecolor). Every board cell sets explicit foreground **and** background so contrast does not depend on the user's terminal theme. Numbers use distinct colours (1 bright blue, 2 green, 3 bright red, 4 magenta, 5 red, 6 cyan, 7 white, 8 dark grey); the palette lives in one table (`theme.go`). |
| U-7 | **Mono mode:** when the `NO_COLOR` environment variable is set, `--no-color` is passed, or the Options colour setting is `off`, emit no colour codes; use only bold/reverse/underline attributes (focused cell = reverse video). The game must remain fully playable. |
| U-8 | **Rendering:** the renderer diffs against the previous frame, emits only changed runs, and writes each frame with a single buffered write. It never writes to the bottom-right cell of the terminal (which would scroll it). It repaints fully on resize and on `Ctrl+L`. |

| Meaning | `ascii` | `unicode` |
|---|---|---|
| Hidden | `#` | `▒` |
| Revealed, 0 adjacent | (space) | (space) |
| Revealed, 1–8 | `1`…`8` | `1`…`8` |
| Flag | `F` | `►` |
| Question mark | `?` | `?` |
| Mine | `*` | `☼` |
| Triggered mine | mine glyph on red (reverse+bold in mono) | same |
| Wrong flag | `x` | `×` |

The Unicode glyphs come from the WGL4 character repertoire (block elements, `►`, `☼`), which Windows fonts such as Consolas and Linux fonts such as DejaVu Sans Mono are expected to cover. **Confirm this visually in M3**; if any glyph fails on a target font, replace it.

| ID | Requirement |
|---|---|
| U-9 | The **status line** always describes the focused cell in words ("Row 3, col 5: hidden / flagged / 3 adjacent mines / mine") and announces game events ("Game started", "You hit a mine", "You won in 42.31 s"). |
| U-10 | **Glyph set:** `--glyphs auto\|ascii\|unicode` (default `auto`, also an Options setting). `auto` picks `unicode` under Windows Terminal (`WT_SESSION` set) or when the first non-empty of `LC_ALL`, `LC_CTYPE`, `LANG` names UTF-8, unless `TERM=linux`; otherwise `ascii`. |
| U-11 | The **hardware cursor** is shown and kept on the focused cell, so screen magnifiers and readers that track the cursor follow play. It is hidden only while a dialog captures input. |

### Mouse

| ID | Requirement |
|---|---|
| U-12 | Mouse input uses SGR reporting. A **gesture** starts at the first button press and ends when all buttons are released. The action applies at the end, on the cell under the release, and only if press and release were on the same cell (otherwise the gesture is cancelled): **left only** = reveal; **right only** = flag; **middle only**, or **left and right both held at any point** = chord. Acting on release is what makes left+right chording possible. |
| U-13 | A gesture that never receives its release (terminal lost focus, dropped event) **MUST NOT** get stuck: any key press, or a second press of an already-held button, resets it. |
| U-14 | **Click-to-chord** (`clickNumberChords`, default **on**): a plain left-click on a revealed number chords. This makes chording possible on trackpads with no middle button. |
| U-15 | Menu-bar items, the face, dialog buttons and list rows are clickable. Wheel events are ignored. Where the mouse is unavailable (Linux virtual console, `tmux` with mouse off) everything is still reachable from the keyboard. |

### Keyboard

| ID | Requirement |
|---|---|
| U-16 | **Board:** arrow keys move the focus (with `Home`/`End` = row start/end, `PgUp`/`PgDn` = top/bottom row); `Space`/`Enter` = reveal (on a revealed number: chord, per U-14); `F` = flag; `C` = chord. **Global:** `N` or `F2` = new game; `D` = difficulty; `B` = best times; `O` = options; `?` or `F1` = help; `Q` = quit (asks to confirm only while a game is in progress); `Ctrl+C` = quit immediately; `Esc` closes a dialog. The whole game **MUST** be playable without a mouse. |

### Dialogs

| ID | Requirement |
|---|---|
| U-17 | **Difficulty:** Beginner / Intermediate / Expert / Custom, chosen by number key, arrows + `Enter`, or click. **Custom** has width/height/mines fields (`Tab` moves between them) with live validation against [G-1](#51-board-and-difficulty) and U-2. |
| U-18 | **Best Times:** top 5 per preset, with a Reset action that asks for confirmation. |
| U-19 | **Options:** question marks, click-to-chord, glyph set, colour (auto/on/off). Changes apply immediately and are saved. |
| U-20 | **End of game:** a banner states win or loss and the time; offers *New game* / *Quit*. On a top-5 win it then offers a name field ([P-4](#8-persistence)). It **SHOULD** show the game's seed so a board can be reproduced with `--seed`. |
| U-21 | **Help:** a one-screen summary of the controls in this section. |

---

## 7. Terminal layer

`internal/term` is the only place that talks to the operating system's terminal facilities. Its Linux half was prototyped and verified ([§3](#3-technology-decision)); its Windows half was compiled only.

| ID | Requirement |
|---|---|
| TL-1 | **Refuse non-terminals:** if stdin or stdout is not a terminal, or `TERM` is `dumb` or unset (Linux), print a one-line message to stderr and exit 2. |
| TL-2 | **Setup**, in this order: enter raw mode; alternate screen (`ESC[?1049h`); disable auto-wrap (`ESC[?7l`); enable mouse press/release (`?1000h`) with SGR encoding (`?1006h`). Raw mode **MUST** deliver Ctrl+C as a key rather than a signal. **Linux:** `TCGETS`/`TCSETS` via `syscall`; clear `ECHO`, `ICANON`, `ISIG`, `IEXTEN`, `ICRNL`, `IXON`, `OPOST`. **Windows:** via `kernel32`: clear `ENABLE_PROCESSED_INPUT`, `ENABLE_LINE_INPUT`, `ENABLE_ECHO_INPUT` and **`ENABLE_QUICK_EDIT_MODE`** (which otherwise swallows mouse clicks), set `ENABLE_VIRTUAL_TERMINAL_INPUT` and `ENABLE_EXTENDED_FLAGS`; on output set `ENABLE_VIRTUAL_TERMINAL_PROCESSING`; set the output code page to UTF-8 (65001). |
| TL-3 | **Restore guarantee:** a single idempotent `Restore()` undoes everything in TL-2 (mouse off, auto-wrap on, cursor shown, alternate screen left, original terminal/console modes and code page) and runs on **every** exit path: normal quit, Ctrl+C key, `SIGTERM`/`SIGHUP`, fatal error, and panic (recover → restore → print the panic to stderr → exit 1). On Windows, console close, logoff and shutdown reach Go as `SIGTERM`. |
| TL-4 | **Input decoding** is a pure function over bytes (`internal/tui`): printable text (UTF-8); `Enter` (CR/LF); `Tab`, `Shift+Tab`; `Backspace` (0x7F/0x08); `Esc`; arrows, `Home`/`End`, `PgUp`/`PgDn`, `Delete` (CSI and SS3 forms); `F1`–`F12`; `Ctrl+letter` (bytes 1–26); and SGR mouse `CSI < b ; x ; y (M\|m)` (low bits: 0 left, 1 middle, 2 right; +4 shift, +8 alt, +16 ctrl, +32 motion, +64 wheel; coordinates are 1-based). Unknown sequences are dropped, never crash (fuzzed, [T-8](#11-testing-strategy)); sequences split across reads are buffered; a lone `Esc` is resolved after 30 ms with no following byte. |
| TL-5 | **Resize:** poll the size every 250 ms on the UI tick (`TIOCGWINSZ` on Linux; `GetConsoleScreenBufferInfo` on Windows) instead of using `SIGWINCH`, so both OSes share one code path. A change triggers a full repaint. |
| TL-6 | **Unsupported platforms** (e.g. macOS in v1) build, but exit 2 with "unsupported platform". |

---

## 8. Persistence

| ID | Requirement |
|---|---|
| P-1 | State lives in one file, `state.json`, in `<config dir>/minesweeper/`, where `<config dir>` is `os.UserConfigDir()` (Linux: `$XDG_CONFIG_HOME` or `~/.config`; Windows: `%AppData%`). `--data-dir` overrides it (`--data-dir .` gives a portable install). |
| P-2 | Schema v1: `{"version":1,"settings":{"questionMarks":false,"clickNumberChords":true,"glyphs":"auto","color":"auto"},"lastBoard":{…},"lastName":"…","bestTimes":{"beginner":[{"name":"…","ms":12340,"at":"<RFC 3339 UTC>"}],"intermediate":[…],"expert":[…]}}`. |
| P-3 | Best times: top **5** per preset, ascending by `ms`; on a tie the earlier record ranks first. Custom boards are never recorded. |
| P-4 | A qualifying win is saved **immediately** under the last-used name (default `Anonymous`), so a crash or closed terminal cannot lose it; the name dialog then renames that record. Names are trimmed, ≤ 20 characters, printable characters only. |
| P-5 | Writes are atomic: write a temp file in the same directory, `fsync`, then `os.Rename` over the target (Go's Windows rename replaces existing files). Settings are written when changed and best times when earned, not only on exit. |
| P-6 | A corrupt or unknown-version file **MUST NOT** crash the game or be overwritten silently: rename it to `state.json.bad-<unix time>`, start with defaults, and show a one-line notice in the status line. The file is read with a 1 MiB cap. |
| P-7 | If the data directory cannot be created or written, the game **MUST** still run, without persistence, and the status line says so. |
| P-8 | The game writes nothing outside the data directory. |

---

## 9. Process lifecycle

### Command-line options

| Flag | Default | Meaning |
|---|---|---|
| `--data-dir PATH` | per-user config dir | Where `state.json` lives. |
| `--seed N` | random | Deterministic games: game *k* uses a seed derived from `N` and *k*. |
| `--glyphs auto\|ascii\|unicode` | `auto` | Glyph set ([U-10](#6-user-interface)). Overrides the saved option. |
| `--no-color` | off | Mono mode ([U-7](#6-user-interface)). The `NO_COLOR` environment variable does the same. |
| `--version` | | Print version and exit 0. |
| `--help` | | Print usage and exit 0. |

Exit codes: `0` normal, `1` runtime error, `2` usage error or unsupported terminal. Errors go to stderr **after** the terminal is restored.

| ID | Requirement |
|---|---|
| L-1 | Quitting is explicit: `Q`, `Ctrl+C`, or closing the terminal window. There is no background process and no port to clean up. |
| L-2 | The Windows build is a **console-subsystem** program (no `-H windowsgui`), so double-clicking the `.exe` opens a console window automatically. |
| L-3 | On Windows, if the process is the only client of its console (double-clicked) and it exits on an **error**, it SHOULD wait for Enter first so the message can be read before the window vanishes. |
| L-4 | On Linux the binary is started from a terminal; double-clicking in a file manager is not supported (file managers do not reliably give programs a terminal). |
| L-5 | One game exists per process. Two copies running at once share one `state.json`; the last writer wins. This is a documented limitation, not something to lock against. |

---

## 10. Non-functional requirements and risks

| ID | Requirement |
|---|---|
| N-1 | **Startup:** first frame within 300 ms of launch on modest hardware. |
| N-2 | **Responsiveness:** input → repainted result under 50 ms at p95 on the largest board that fits (50×30). Engine reveal with full flood fill under 5 ms. |
| N-3 | **Size:** each release binary ≤ 5 MB (the probe measured 1.4–1.5 MB before the game code). |
| N-4 | **Memory:** target under 30 MB resident (a budget to confirm at M5, not yet measured). |
| N-5 | **Go version:** `go.mod` declares `go 1.22` (`math/rand/v2` stable PCG source, built-in `min`/`max`). CI also builds with the latest stable Go. |
| N-6 | **Terminals:** *Windows* — Windows Terminal, the classic console host on Windows 10 1809+, and the VS Code terminal. *Linux* — any xterm-compatible terminal with SGR mouse (GNOME Terminal/VTE, Konsole, xterm, kitty, alacritty, foot), also over SSH and inside `tmux` with `mouse on`. The Linux virtual console has no mouse: keyboard-only, `ascii` glyphs. |
| N-7 | **Strings:** all user-visible text lives in one `strings` table so a later translation touches one place. |
| N-8 | **Output:** nothing is printed to stdout/stderr while the UI is up; fatal errors print once, after the terminal is restored. |

### Risks

| ID | Risk | Mitigation |
|---|---|---|
| R-1 | **Windows console input is unverified.** Mouse events via virtual-terminal input have not been run on real Windows (classic console host vs Windows Terminal may differ). | Do the terminal layer **first** (M2) and test it on real Windows 10 and 11 before building the game on top. Fallback: read native console input records (`ReadConsoleInputW`) inside `internal/term` and emit the same Events — nothing above `term` changes. Keyboard play works regardless. |
| R-2 | Unicode glyphs may not exist in a user's terminal font. | ASCII set is always available; `auto` is conservative; glyph check is part of M3. |
| R-3 | Terminal themes (e.g. Solarized) remap the 16 colours, weakening contrast. | Explicit fg+bg on every cell; `--no-color` mono mode. |
| R-4 | Large boards need large terminals. | U-2: clear "too small" message and Custom-dialog limits. |
| R-5 | Some environments deliver no mouse events (tmux mouse off, Linux console, some multiplexers). | Complete keyboard control (U-16). |

---

## 11. Testing strategy

Because the UI is a pipeline of pure steps ([A-2](#4-architecture)), nearly everything is testable with plain `go test` and no terminal.

| ID | Area | Required tests |
|---|---|---|
| T-1 | Engine — first click | For thousands of seeds and every board size/corner/edge/centre click, including maximum density (`mines = W×H − 9`): first reveal never hits a mine and always produces a `0`. |
| T-2 | Engine — placement | Exact mine count; every cell's number equals a brute-force recount; determinism for equal seed + first click; golden layouts for several seeds ([G-5](#52-mine-placement)). |
| T-3 | Engine — flood fill | Matches a naive reference implementation on small boards; a sparse 50×30 board completes without deep recursion; flags block fill. |
| T-4 | Engine — actions and end states | Table-driven cases for reveal, flag cycle (with/without `?`), chord (exact, too few, too many flags, wrong flag → loss, on a `0`, on a hidden cell), post-game rejection; win detection, loss reveal, negative counter, timer start/stop/cap using a fake clock. |
| T-5 | UI model | Feed Event sequences to `Model.Update`, assert on `View()` text: a full win, a full loss, flag cycle, chord, menus and dialogs, Custom validation, name entry, "too small" and recovery, status-line text. Golden frames for a fixed seed on Beginner. |
| T-6 | Mouse gestures | Left, right, middle, left+right (both orders), press/release on different cells (cancelled), lost release (reset by key press), click-to-chord on/off. |
| T-7 | Renderer | Diff of identical frames is empty; one changed cell yields minimal output; applying a diff to a tiny in-test screen model reproduces the next frame; last cell is never written. |
| T-8 | Input decoder | Table tests for every sequence in [TL-4](#7-terminal-layer), including split reads and the lone-`Esc` timeout; a **`go test -fuzz` target** asserting the decoder never panics and always makes progress. |
| T-9 | Store | Temp dir: round trip; atomic write leaves no partial file; corrupt file → `.bad-*` backup and defaults; unwritable dir → runs without persistence; top-5 ordering and tie-break. |
| T-10 | Real-terminal integration (Linux CI) | Using only the standard library (`/dev/ptmx` via `syscall`), start the built binary in a pseudo-terminal, send keys and SGR mouse bytes, and assert: alternate screen and mouse mode on/off, a move changes the frame, and **the terminal's settings are identical before and after** a normal quit, a Ctrl+C key, and `SIGTERM`. This is the same check that was run against the prototype. |
| T-11 | Windows | `GOOS=windows go vet ./...` and cross-build in CI; behaviour verified by the manual checklist below on real machines (R-1). |

**Gates (CI):** `gofmt -l .` empty · `go vet ./...` for `linux` and `windows` · `go test ./...` on Linux **and** Windows · `go test -race ./...` on Linux · no `require` in `go.mod` · statement coverage: engine ≥ 95 %, `tui` ≥ 85 %, overall ≥ 80 %.

---

## 12. Acceptance checklist

Run manually on a **clean** Windows 10 (classic console host), Windows 11 (Windows Terminal) and a mainstream Linux desktop (no Go, Python or dev tools installed), using only the downloaded release file.

- [ ] Runs with no installer and no extra downloads; on Windows, double-click opens a console window with the game.
- [ ] Beginner, Intermediate, Expert and a Custom board all have the right size and mine count; Expert fits a default 80×24 terminal.
- [ ] First click never loses and always opens a region, including in a corner.
- [ ] Left, right, middle and left+right clicks behave as in [U-12](#6-user-interface); click-to-chord works on a trackpad; on Windows **mouse clicks are not swallowed by QuickEdit** (TL-2).
- [ ] Playable entirely from the keyboard ([U-16](#6-user-interface)); the hardware cursor tracks the focused cell.
- [ ] Win → `B)`, auto-flagged mines, counter 0, name prompt on a top-5 time; loss → `X(`, mines / wrong flags / triggered mine shown.
- [ ] Timer starts on first reveal and stops at the end.
- [ ] Best times and settings persist across restarts; the file is where [P-1](#8-persistence) says and nothing else is written elsewhere.
- [ ] Corrupt `state.json` → game still starts, backup file created, notice shown.
- [ ] After **every** way of leaving (Q, Ctrl+C, closing the window, `kill`), the shell is back to normal: echo on, cursor visible, no mouse garbage typed on click, scrollback intact.
- [ ] Resizing smaller shows the "too small" screen and recovers on enlarging; no stray characters.
- [ ] Both glyph sets render correctly in each terminal's default font; `NO_COLOR` / `--no-color` is fully playable; a light-background terminal and a dark one are both readable.
- [ ] Works over SSH and in `tmux` (with `mouse on`).
- [ ] With the network disabled, everything works; `ss -tunap` / `netstat` shows no sockets opened by the game (C-5).

---

## 13. Milestones

Update the checkboxes here (and the README status line) as work lands. The order is **risk-first**: the unverified Windows terminal work comes before the game is built on top of it.

| | Milestone | Exit criteria |
|---|---|---|
| [ ] | **M0 Scaffold** | `go.mod` (no requires), `cmd/minesweeper` prints `--version`, CI green on Ubuntu + Windows, cross-builds for all four targets. |
| [ ] | **M1 Engine** | §5 fully implemented; T-1 … T-4 pass; coverage ≥ 95 %. |
| [ ] | **M2 Terminal layer + decoder + renderer** | `internal/term` (§7) and the input decoder/renderer working with a throwaway demo that draws a grid and reports clicks and keys. T-7, T-8, T-10 pass; **demo verified on real Windows 10 and 11 and on Linux (R-1 closed or fallback adopted).** |
| [ ] | **M3 Playable game** | Model + view for core play (reveal, flag, chord, win/lose, counters, face, keyboard, mouse gestures, too-small screen); T-5, T-6 pass; glyph check on target fonts (U-5). |
| [ ] | **M4 Full UI + persistence** | All dialogs (U-17…U-21), Options, Best Times, Custom, help, §8; T-9 passes. |
| [ ] | **M5 Release** | `go run ./tools/build` produces all four binaries + `SHA256SUMS`; release workflow on tags; NFRs measured; §12 checklist passed; README screenshots. |

### Build and release (M0 / M5)

| ID | Requirement |
|---|---|
| B-1 | Build tooling is written in **Go** (`go run ./tools/build`), not `make` or shell scripts, so the identical command works on Windows and Linux without extra installs. |
| B-2 | Release builds use `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=<v>"`, version from `git describe --tags --always --dirty` (fallback `dev`). |
| B-3 | Targets: `linux/amd64`, `linux/arm64`, `windows/amd64`, `windows/arm64`. Other platforms build but exit "unsupported platform" ([TL-6](#7-terminal-layer)). |
| B-4 | Output names: `minesweeper-<version>-<os>-<arch>[.exe]` in `dist/`, plus `SHA256SUMS`. |
| B-5 | Pushing a tag `vX.Y.Z` (semver) triggers a workflow that builds all targets and attaches the files to a GitHub Release. |
| B-6 | Binaries are **unsigned**; Windows SmartScreen will warn on first run. The README documents this. Code signing is out of scope. |

---

## 14. Out of scope and future work

Not in v1; candidates in rough priority order:

1. **Native-window front-end** with hand-written Win32 and X11 backends reusing `internal/engine` — the runner-up in §3. Worth doing only if a terminal proves unacceptable; needs a Windows test machine and has no Wayland-only support.
2. **Scrolling viewport** so boards larger than the terminal can be played.
3. **No-guess board generator** and a hint/solver built on the engine.
4. Save and resume an in-progress game.
5. 256-colour / truecolor themes.
6. Localisation (see N-7).
7. Replay/export of a game from its seed and move list.
