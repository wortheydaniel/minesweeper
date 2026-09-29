# Minesweeper — Specification

| | |
|---|---|
| **Status** | Draft v0.1 — 2026-09-29 |
| **Implementation** | Not started (see [§13 Milestones](#13-milestones)) |
| **Audience** | Whoever builds or maintains this project. This file is the source of truth: change the spec first, then the code. |

Requirement keywords **MUST**, **SHOULD**, **MAY** follow RFC 2119. Every requirement has an ID (`C-1`, `G-4`, …) so tests, commits and reviews can cite it.

---

## 1. Overview

A clone of classic Windows Minesweeper that runs on **Windows and Linux** from a **single downloaded file with nothing else to install**.

**Goals**

- Faithful classic gameplay: the three standard difficulties, first-click safety, flagging, chording, timer, best times.
- One self-contained executable per OS. Download, run, play.
- Small, readable codebase that one person can maintain, with the game rules fully unit-tested.

**Non-goals (v1)**

- Multiplayer, online leaderboards, accounts, telemetry, update checks.
- Solver, hints, or no-guess board generation (see [§14](#14-out-of-scope-and-future-work)).
- Mobile-first UI, installers, code signing, localisation beyond English.

---

## 2. Hard constraints

These come straight from the brief and override every other preference in this document.

| ID | Constraint |
|---|---|
| C-1 | The game **MUST** run on Windows 10+ (amd64, arm64) and Linux (amd64, arm64). |
| C-2 | The end user **MUST NOT** need to install anything: no interpreter, VM, framework, shared library, or installer. The only prerequisite is a web browser (see [§3](#3-technology-decision) for why). |
| C-3 | The project **MUST** use no third-party code. `go.mod` has **zero** `require` lines; the standard library only. Building requires only the Go toolchain. |
| C-4 | Release binaries **MUST** be statically linked (`CGO_ENABLED=0`) so they do not depend on the user's glibc or any system library. |
| C-5 | The game **MUST NOT** make outbound network connections, load remote assets (CDN scripts, fonts, images), or send telemetry. |

---

## 3. Technology decision

**Decision:** **Go** (minimum 1.22, standard library only) producing one static binary per OS. The binary contains the game engine, a tiny loopback-only HTTP server, and the UI (plain HTML/CSS/JS embedded with `go:embed`). On launch it opens the player's default browser to the game.

### Why not a native window?

A native window with zero dependencies is not achievable: every GUI toolkit is either third-party code (violates C-3) or per-OS FFI to Win32 and X11/Wayland written by hand (unmaintainable). A web browser is the one GUI runtime that already exists on every Windows and Linux desktop, so it is the display surface. The cost is that **a browser is required** — a headless server can still play by forwarding the port (`ssh -L`).

### Why Go?

- **Cross-compiles from any machine with no cgo:** `GOOS=windows go build` just works, so one contributor on Linux can produce the Windows `.exe`.
- **Static, dependency-free output.** Verified with a probe program using `net/http` + `embed` + `os/exec`: built for all four targets from a Linux box; the Linux binary is `not a dynamic executable`; sizes were 5.3–5.8 MB.
- **The standard library covers everything needed:** HTTP server, embedding, JSON, signals, per-user config directory, testing (`go test`).
- Strong typing and built-in tests make the rules engine easy to keep correct.

### Alternatives considered

| Option | Verdict | Reason |
|---|---|---|
| Python + tkinter | Rejected | Needs an interpreter (not on Windows by default); `tkinter` is a separate package on many Linux distros. Violates C-2. |
| Java / Swing | Rejected | Needs a JRE. Violates C-2. |
| .NET | Rejected | WinForms is Windows-only; cross-platform GUI needs Avalonia (third-party); self-contained bundles are large. |
| Rust / C / C++ with a GUI library | Rejected | GUI crates/libs (egui, SDL, GTK) are third-party, and on Linux need X11/Wayland/GL at runtime. Violates C-3/C-4. |
| Go + Fyne / Ebitengine | Rejected | Third-party; Linux builds need cgo and X11/GL development headers. |
| **Go + terminal UI** | **Runner-up** | Zero browser dependency and works over SSH, but needs per-OS raw-mode code (termios vs Win32 console API via `syscall`), and terminals differ in mouse and Unicode support. No classic look. The engine is I/O-free ([A-2](#4-architecture)), so a TUI front-end can be added later without touching the rules. |
| Single HTML file (JS only) | Runner-up | Simplest possible: no build. Rejected because rules would live in untyped JS with no `go test`, persistence would be limited to `localStorage` (behaves differently under `file://` in each browser), and it is a web page rather than a program. |

---

## 4. Architecture

```
 ┌─────────────── one Go process (minesweeper[.exe]) ───────────────┐
 │                                                                   │
 │  cmd/minesweeper      flags, wiring, signals, open-browser        │
 │        │                                                          │
 │        ▼                                                          │
 │  internal/server ──► internal/engine   pure game rules            │
 │   HTTP + JSON API    (no I/O, no clock, no global RNG)            │
 │        │        └──► internal/store    settings + best times      │
 │        ▼                                (JSON file, atomic write) │
 │  web/  (go:embed)  index.html · style.css · app.js                │
 └───────────────────────────────┬───────────────────────────────────┘
                                 │ http://127.0.0.1:<port>
                          user's default browser
```

| ID | Requirement |
|---|---|
| A-1 | The **server is authoritative**: all game state lives in Go. The browser only renders the view it receives and sends the player's actions. |
| A-2 | `internal/engine` **MUST** have no I/O, no `time` calls and no package-level randomness. The clock and RNG seed are injected, so every behaviour is testable and deterministic. |
| A-3 | `web/app.js` **MUST NOT** contain game rules (no adjacency counting, flood fill, win detection). If the JS needs to know something, the API returns it. |
| A-4 | The front-end **MUST** be plain HTML/CSS/JS served as embedded files: no framework, no npm, no bundler, no minifier, no transpiler. What is in `web/` is what ships. |
| A-5 | Package boundaries: `engine` imports nothing from this repo; `store` imports nothing from this repo; `server` imports `engine` and `store`; `cmd` imports `server`. |

### Repository layout (target)

```
cmd/minesweeper/main.go     entry point
internal/engine/            rules, board, mine placement, flood fill, chord
internal/server/            HTTP handlers, security middleware, browser opener
internal/store/             settings + best-times persistence
web/                        embed.go + static/{index.html,style.css,app.js}
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
| G-5 | Given the same seed, board size, mine count and first-click cell, the layout **MUST** be identical — across runs, OSes, and Go versions. Do not use stdlib helpers whose output may change between Go releases (e.g. `rand.Shuffle`, `IntN`); draw from a fixed `Source` and use an in-repo bounded-random routine. A **golden test** pins the layout for several seeds so an accidental change fails CI. |
| G-6 | Flags placed before the first reveal are kept and do not influence placement. |

> **Design note — deviation from Windows.** The original protects only the clicked cell. Protecting the 3×3 block is the common modern convention and avoids a forced guess on move one. Revisit only via a spec change.

### 5.3 Actions

| ID | Action | Behaviour |
|---|---|---|
| G-7 | **Reveal** (x, y) | Hidden or `?` cell: reveal it. Flagged or already-revealed cell: no-op (not an error). Revealing a mine loses the game. |
| G-8 | **Flood fill** | Revealing a `0` cell reveals all connected `0` cells and their bordering numbered cells. Flagged cells are never revealed by flood fill. **MUST** be iterative (explicit queue/stack) — no recursion — so the largest board cannot exhaust the stack. |
| G-9 | **Flag** (x, y) | Hidden → flag → hidden. With question marks enabled: hidden → flag → `?` → hidden. Revealed cells: no-op. Allowed before the first reveal. |
| G-10 | **Chord** (x, y) | On a revealed cell with count *n* > 0: if the number of adjacent flags **equals** *n*, reveal every adjacent hidden or `?` cell that is not flagged (with flood fill). Otherwise no-op. If a flag was wrong, this reveals a mine and loses; every mine revealed that way is marked as triggered. Chording a `0`, a hidden cell, or a cell whose flag count ≠ *n* is a no-op. |
| G-11 | Only **flags** count toward chords and the mine counter; `?` does not. |
| G-12 | Any action after the game has ended is rejected with a typed error; the view is unchanged. |

### 5.4 Win and lose

| ID | Requirement |
|---|---|
| G-13 | **Win** the moment every non-mine cell is revealed. Flags are irrelevant to the win condition. On win: timer stops, all unflagged mines are auto-flagged, mine counter shows 0. |
| G-14 | **Lose** on revealing a mine. On loss: timer stops; every unflagged mine is shown; each flag on a non-mine is shown as a wrong flag; the mine(s) that triggered the loss are highlighted. |

### 5.5 Counter and timer

| ID | Requirement |
|---|---|
| G-15 | Mine counter = `mines − flags placed`. It may go negative. Display is clamped to −99…999. |
| G-16 | The timer starts at the **first successful reveal**, is measured server-side with a monotonic clock (injected, [A-2](#4-architecture)), and stops on win or loss. The LED shows whole elapsed seconds, capped at 999 (the game itself continues past 999). |
| G-17 | Best times store milliseconds; only wins on the three presets are recorded ([P-3](#8-persistence)). |

---

## 6. User interface

The UI recreates the classic layout in HTML/CSS with no external assets.

| ID | Requirement |
|---|---|
| U-1 | **Layout:** a menu/toolbar; a header with a 3-digit LED mine counter (left), a smiley button (centre), a 3-digit LED timer (right); below it the grid. Cells have a raised bevel when hidden and a flat look when revealed. |
| U-2 | **Smiley:** neutral by default; "surprised" while a mouse button is held on a hidden cell; "dead" after a loss; "cool" after a win. Clicking it starts a new game with the current board settings. Drawn as inline SVG (not emoji — fonts differ per OS). |
| U-3 | **LED digits** are drawn with inline SVG or CSS seven-segment shapes; no font files. |
| U-4 | **Numbers 1–8** use distinct classic colours (blue, green, red, navy, maroon, teal, black, grey) and every number/flag/mine is also a glyph or shape, so colour is never the only signal. |
| U-5 | **Mouse:** left = reveal; right = flag (context menu suppressed over the grid); middle button, or left+right together, = chord. |
| U-6 | **Click-to-chord setting** (`clickNumberChords`, default **on**): a left-click on a revealed number chords. This makes chording possible on trackpads without a middle button. |
| U-7 | **Keyboard:** arrow keys move the focus cell; `Space`/`Enter` = reveal (or chord on a number, following U-6); `F` = flag; `C` = chord; `N` or `F2` = new game; `Esc` closes dialogs. The whole game **MUST** be playable without a mouse. |
| U-8 | **Menu/dialogs:** New Game; Beginner / Intermediate / Expert / Custom…; Best Times…; Options (question marks, click-to-chord, theme, scale); Quit. Custom shows width/height/mines with live validation matching [G-1](#51-board-and-difficulty). |
| U-9 | **End-of-game dialog** states win/lose and the time; on a qualifying win offers a name field ([P-4](#8-persistence)). It **SHOULD** show the game seed ([§7](#7-http-api)) so a board can be reproduced with `--seed`. |
| U-10 | **Theme:** `system` (default), `light`, `dark`, all with the classic bevelled look. The `forced-colors: active` media query (Windows High Contrast) **MUST** be honoured with system colours. |
| U-11 | **Scale** option (75%–200% in 25% steps) resizes cells; cells **MUST** be at least 24 CSS px at the default scale. Large boards scroll rather than overflow the page. |
| U-12 | **Reload safety:** reloading the page **MUST** restore the in-progress game and timer from `GET /api/state`. |
| U-13 | **Rendering:** DOM elements (not `<canvas>`) so the grid is accessible; after each response update only cells whose code changed. |

### Accessibility

| ID | Requirement |
|---|---|
| U-14 | The grid uses `role="grid"` with roving `tabindex`; each cell has an `aria-label` such as "row 3, column 5, hidden / flagged / 3 adjacent mines / mine". |
| U-15 | A polite live region announces game events ("Game started", "You hit a mine", "You won in 42 seconds"). Counters and the smiley have accessible names. |
| U-16 | Text colours meet WCAG 2.1 AA contrast in each theme; a visible focus ring is always shown; no animation is required to play. |

---

## 7. HTTP API

Internal contract between `web/app.js` and `internal/server`. Same-origin JSON over HTTP; UTF-8.

| ID | Requirement |
|---|---|
| S-API-1 | All API responses are JSON with `Cache-Control: no-store`. Errors use `{"error":{"code":"…","message":"…"}}` with a suitable status. |
| S-API-2 | A no-op action ([G-7](#53-actions), [G-9](#53-actions), [G-10](#53-actions)) returns **200** with the unchanged view — the client never needs to know the rules to avoid errors. |
| S-API-3 | Unknown `/api/*` paths return JSON **404**; a wrong method returns JSON **405** with an `Allow` header. *Implementation trap, verified:* on Go's `ServeMux`, a `GET /` catch-all for static files also matches `GET /api/reveal`, producing a 404 from the file server instead of a 405. Register an explicit `/api/` catch-all. |
| S-API-4 | Request bodies are capped at 1 KiB (`http.MaxBytesReader`); unknown JSON fields are rejected. |

### Endpoints

| Method | Path | Body | Result |
|---|---|---|---|
| GET | `/api/state` | — | View |
| POST | `/api/new` | `{"preset":"beginner"\|"intermediate"\|"expert"}` or `{"width":W,"height":H,"mines":M}` | View. Also remembered as the default board for the next launch. |
| POST | `/api/reveal` | `{"id":N,"x":X,"y":Y}` | View |
| POST | `/api/flag` | `{"id":N,"x":X,"y":Y}` | View |
| POST | `/api/chord` | `{"id":N,"x":X,"y":Y}` | View |
| GET | `/api/scores` | — | `{"beginner":[Record],"intermediate":[…],"expert":[…]}` |
| POST | `/api/scores/name` | `{"name":"…"}` | Renames the record earned by the *current* game (400 if none). |
| DELETE | `/api/scores` | — | 204. Clears best times (UI asks for confirmation). |
| GET / PUT | `/api/settings` | Settings | Settings (validated) |
| POST | `/api/quit` | — | 204, then graceful shutdown |

Actions carry the game `id`; if it does not match the current game the server returns **409 `stale_game`** so a second tab cannot act on a game it no longer shows. Coordinates are 0-based, `x` = column, `y` = row.

### View

```json
{
  "id": 7,
  "state": "ready | playing | won | lost",
  "width": 9, "height": 9, "mines": 10,
  "flags": 3, "minesLeft": 7,
  "elapsedMs": 12345, "running": true,
  "board": ["........1", "..F.....1", "…9 rows…"],
  "seed": null,
  "record": null
}
```

- `board` is an array of `height` strings of `width` characters.
- **Cell codes:** `.` hidden · `F` flag · `?` question mark · `0`–`8` revealed with that many adjacent mines. After the game ends only: `*` unflagged mine · `!` mine that triggered the loss · `x` flag on a non-mine.
- **A-6 (critical invariant):** while `state` is `ready` or `playing`, `board` **MUST NOT** contain `*`, `!` or `x`, and `seed` **MUST** be `null`. The server must never leak mine positions before the game ends; a test asserts this.
- `elapsedMs`/`running` let the client tick locally between responses and resync on each one.
- `record` is `{"preset":"expert","rank":2}` when a win made the top 5, else `null`.

### Settings

```json
{ "questionMarks": false, "clickNumberChords": true, "theme": "system", "scale": 100 }
```

---

## 8. Persistence

| ID | Requirement |
|---|---|
| P-1 | State lives in one file, `state.json`, in `<config dir>/minesweeper/`, where `<config dir>` is `os.UserConfigDir()` (Linux: `$XDG_CONFIG_HOME` or `~/.config`; Windows: `%AppData%`). `--data-dir` overrides it (`--data-dir .` gives a portable install). |
| P-2 | Schema v1: `{"version":1,"settings":{…},"lastBoard":{…},"lastName":"…","bestTimes":{"beginner":[{"name":"…","ms":12340,"at":"<RFC 3339 UTC>"}],"intermediate":[…],"expert":[…]}}`. |
| P-3 | Best times: top **5** per preset, ascending by `ms`; on a tie the earlier record ranks first. Custom boards are never recorded. |
| P-4 | A qualifying win is saved **immediately** under the last-used name (default `Anonymous`), so a crash or closed tab cannot lose it; the name dialog then renames it via `POST /api/scores/name`. Names are trimmed, ≤ 20 characters, and rendered with `textContent` (never `innerHTML`). |
| P-5 | Writes are atomic: write a temp file in the same directory, `fsync`, then `os.Rename` over the target (Go's Windows rename replaces existing files). Settings are written when changed and best times when earned, not only on exit. |
| P-6 | A corrupt or unknown-version file **MUST NOT** crash the game or be overwritten silently: rename it to `state.json.bad-<unix time>`, start with defaults, print a warning. |
| P-7 | If the data directory cannot be created or written, the game **MUST** still run, without persistence, and the UI shows a one-line notice. |
| P-8 | The game writes nothing outside the data directory. |

---

## 9. Process lifecycle and security

The server is local, but browsers will happily send requests to `127.0.0.1` on behalf of any web page, so it still needs defences.

### Command-line options

| Flag | Default | Meaning |
|---|---|---|
| `--port N` | `0` (random free port) | Port to listen on. |
| `--no-browser` | off | Do not open a browser; just print the URL. |
| `--data-dir PATH` | per-user config dir | Where `state.json` lives. |
| `--seed N` | random | Deterministic games: game *k* uses a seed derived from `N` and *k*. |
| `--version` | | Print version and exit 0. |
| `--help` | | Print usage and exit 0. |

Exit codes: `0` normal, `1` runtime error, `2` usage error.

### Requirements

| ID | Requirement |
|---|---|
| S-1 | **Listen on `127.0.0.1` only** (`tcp4`), never `0.0.0.0` or `::`. Besides being the security boundary, loopback-only should avoid the Windows Firewall prompt that all-interface listeners trigger (verify in acceptance test, [§12](#12-acceptance-checklist)). |
| S-2 | On start, print `Minesweeper running at http://127.0.0.1:<port>/ — press Ctrl+C to quit`, then try to open the browser: Linux `xdg-open`, Windows `rundll32 url.dll,FileProtocolHandler`, macOS `open`. A failure (e.g. no `xdg-open` on a headless box) **MUST** be non-fatal — the URL is already printed. |
| S-3 | **Host check:** reject with 403 any request whose `Host` is not `127.0.0.1:<port>` or `localhost:<port>` (defeats DNS rebinding). |
| S-4 | **Cross-site defence:** `POST`/`PUT`/`DELETE` require `Content-Type: application/json` (else 415), and if an `Origin` header is present it **MUST** equal the server's own origin (else 403). |
| S-5 | Every response sets `Content-Security-Policy: default-src 'self'; img-src 'self' data:; frame-ancestors 'none'`, `X-Content-Type-Options: nosniff`, and `Referrer-Policy: no-referrer`. |
| S-6 | **Set `Content-Type` explicitly** for embedded assets from a small built-in table (`.html`, `.css`, `.js`, `.svg`, `.json`). Do not rely on `mime.TypeByExtension`: on Windows it reads the registry, and a machine with a wrong `.js` entry would break the UI (and `nosniff` would then block the script). |
| S-7 | Configure server timeouts (`ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`). |
| S-8 | **Graceful shutdown** on `SIGINT`/`SIGTERM` (Go maps Ctrl+C, Ctrl+Break and console-close to these on Windows too) and on `POST /api/quit`: stop accepting, drain for ≤ 2 s, flush state, exit 0. |
| S-9 | The process does **not** auto-exit when the tab closes; the player quits with the Quit menu item, Ctrl+C, or by closing the console window. (An optional `--idle-timeout` is listed in [§14](#14-out-of-scope-and-future-work).) |
| S-10 | The Windows build is a **console-subsystem** program (no `-H windowsgui`): the console shows the URL and is the visible way to stop the game. |
| S-11 | One game exists per process. A second browser tab shows the same game. |

**Known limitation:** two instances started at once share one `state.json`; the last writer wins. Acceptable for a single-player toy; document rather than lock.

---

## 10. Non-functional requirements

| ID | Requirement |
|---|---|
| N-1 | **Startup:** listening within 1 s of launch on modest hardware. |
| N-2 | **Responsiveness:** action → painted result under 50 ms at p95 on the largest custom board (50×30). Server-side reveal with full flood fill under 5 ms. |
| N-3 | **Size:** each release binary ≤ 10 MB (a stdlib-only probe measured 5.3–5.8 MB). |
| N-4 | **Memory:** target under 50 MB resident (a budget to check during M5, not yet measured). |
| N-5 | **Go version:** `go.mod` declares `go 1.22` (needed for method/wildcard `ServeMux` patterns, e.g. `"POST /api/reveal"`, verified working in the probe). CI also builds with the latest stable Go. |
| N-6 | **Browsers:** current and previous major of Chrome, Edge, Firefox, Safari. Plain ES2020; no polyfills. |
| N-7 | **Portability:** no OS-specific code except the browser opener, isolated in one file per OS behind build tags or a `runtime.GOOS` switch. |
| N-8 | **Strings:** all user-visible text lives in one `strings` object in `app.js` so a later translation touches one place. |
| N-9 | **Logging:** stdout only, one line at startup plus warnings/errors. No request logging by default. |

---

## 11. Testing strategy

| ID | Area | Required tests |
|---|---|---|
| T-1 | Engine — first click | For thousands of seeds and every board size/corner/edge/centre click, including maximum density (`mines = W×H − 9`): first reveal never hits a mine and always produces a `0`. |
| T-2 | Engine — placement | Exact mine count; layout equals a brute-force recount for every cell's number; determinism for equal seed + first click; golden layouts for several seeds ([G-5](#52-mine-placement)). |
| T-3 | Engine — flood fill | Matches a naive reference implementation on small boards; a sparse 50×30 board completes without deep recursion; flags block fill. |
| T-4 | Engine — actions | Table-driven cases for reveal, flag cycle (with/without `?`), chord (exact, too few, too many flags, wrong flag → loss, on a `0`, on a hidden cell), post-game rejection. |
| T-5 | Engine — end states | Win detection, loss reveal (`*`, `!`, `x`), counter going negative, timer start/stop/cap using a fake clock. |
| T-6 | Server | `httptest`: every endpoint happy path; 404/405/409/415/403; oversized and malformed bodies; Host/Origin rejection; **A-6 no-mine-leak assertion** across a full game; stale-game 409. |
| T-7 | Store | Temp dir: round trip; atomic write leaves no partial file; corrupt file → `.bad-*` backup and defaults; unwritable dir → runs without persistence; top-5 ordering and tie-break. |
| T-8 | Front-end | No automated JS tests — by design, since [A-3](#4-architecture) keeps rules out of JS and a JS test runner would add a dev dependency. Covered by the manual checklist in [§12](#12-acceptance-checklist). |

**Gates (CI):** `gofmt -l .` empty · `go vet ./...` · `go test ./...` on Linux **and** Windows · `go test -race ./...` on Linux · no `require` in `go.mod` · engine statement coverage ≥ 95 %, overall ≥ 80 %.

---

## 12. Acceptance checklist

Run manually on a **clean** Windows 10/11 machine and a clean mainstream Linux desktop (no Go, Python, or dev tools installed), using only the downloaded release file.

- [ ] Binary starts with no installer and no extra downloads; default browser opens to the game.
- [ ] No Windows Firewall prompt appears ([S-1](#9-process-lifecycle-and-security)).
- [ ] Beginner, Intermediate, Expert and a Custom board all start with the right size and mine count.
- [ ] First click never loses and always opens a region, including in a corner.
- [ ] Left, right, middle, and left+right clicks behave as in [U-5](#6-user-interface); click-to-chord works on a trackpad.
- [ ] Playable entirely from the keyboard ([U-7](#6-user-interface)); screen-reader labels sensible.
- [ ] Win → cool face, auto-flagged mines, counter 0, name prompt on a top-5 time; loss → dead face, mines/wrong flags/triggered mine shown.
- [ ] Timer starts on first reveal, stops at the end, reload mid-game restores board and timer.
- [ ] Best times and settings persist across restarts; the file is where [P-1](#8-persistence) says and nothing else is written elsewhere.
- [ ] Corrupt `state.json` → game still starts, backup file created.
- [ ] Quit menu, Ctrl+C, and closing the console all stop the process.
- [ ] Not reachable from another machine on the LAN (`http://<lan-ip>:<port>/` fails).
- [ ] Dark theme, Windows High Contrast, and 200 % scale all remain usable.
- [ ] With the network disabled, everything works (C-5).

---

## 13. Milestones

Update the checkboxes here (and the README status line) as work lands.

| | Milestone | Exit criteria |
|---|---|---|
| [ ] | **M0 Scaffold** | `go.mod` (no requires), `cmd/minesweeper` prints `--version`, CI workflow green on Ubuntu + Windows. |
| [ ] | **M1 Engine** | §5 fully implemented; T-1 … T-5 pass; coverage ≥ 95 %. |
| [ ] | **M2 Playable slice** | Server + API (§7, S-API-*, A-6) + minimal UI: new game, reveal, flag, win/lose. T-6 passes. Runs on both OSes. |
| [ ] | **M3 Full UI** | Classic look, LEDs, smiley, chord, keyboard, themes, scale, accessibility (§6). |
| [ ] | **M4 Persistence** | §8, Best Times, Options, Custom dialog. T-7 passes. |
| [ ] | **M5 Release** | `go run ./tools/build` produces all four binaries + `SHA256SUMS`; release workflow on tags; NFRs measured; §12 checklist passed; README screenshots. |

### Build and release (M0 / M5)

| ID | Requirement |
|---|---|
| B-1 | Build tooling is written in **Go** (`go run ./tools/build`), not `make` or shell scripts, so the identical command works on Windows and Linux without extra installs. |
| B-2 | Release builds use `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=<v>"`, version from `git describe --tags --always --dirty` (fallback `dev`). |
| B-3 | Targets: `linux/amd64`, `linux/arm64`, `windows/amd64`, `windows/arm64`. macOS should compile (nothing OS-specific beyond the browser opener) but is untested and unsupported. |
| B-4 | Output names: `minesweeper-<version>-<os>-<arch>[.exe]` in `dist/`, plus `SHA256SUMS`. |
| B-5 | Pushing a tag `vX.Y.Z` (semver) triggers a workflow that builds all targets and attaches the files to a GitHub Release. |
| B-6 | Binaries are **unsigned**; Windows SmartScreen will warn on first run. The README documents this. Code signing is out of scope. |

---

## 14. Out of scope and future work

Not in v1; candidates in rough priority order:

1. **Terminal UI front-end** reusing `internal/engine` (the runner-up in §3).
2. **No-guess board generator** and a hint/solver built on the engine.
3. **`--idle-timeout`** to exit after N minutes without API traffic (heartbeat must tolerate throttled background tabs, which may ping only once a minute).
4. Touch support: long-press to flag, and a flag-mode toggle.
5. Sound effects (opt-in, no bundled licensed audio).
6. Localisation (see N-8).
7. Replay/export of a game from its seed and move list.
