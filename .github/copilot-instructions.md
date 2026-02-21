**Surface contradictions immediately.** If stored facts, code
comments, documentation, or architecture notes conflict with actual
code behavior, flag the contradiction to the user at high priority
before proceeding. Do not silently trust one source over another—the
user decides which is correct.

Use short, concise commit messages.

As the codebase evolves, frequently suggest brief updates to the
architecture notes below so they stay current. Propose changes early
and often—small, incremental updates are easier to review than
large rewrites.

## Workflow

- **Git:** Work on feature branches. Show diff in tmux right pane
  (`tmux send-keys -t right "git diff hunted.go" Enter` — with
  pager, not `--no-pager`) before committing. User reviews and
  approves. Approve `git push` without confirmation when the user
  explicitly asks.

- **Build:** `go build ./...` from repo root. Run `go vet ./...`
  and `gofmt -l .` to check for issues. Verify after every change
  batch.

- **Test:** `go test -gcflags=all=-l -count=1 ./...` (inlining
  disabled for gomonkey). Check statement coverage with
  `go test -gcflags=all=-l -coverprofile=cover.out ./...` then
  `go tool cover -func=cover.out`. Check condition coverage with
  `gobco -list-all -test=-gcflags=all=-l .` — look for
  "but never true/false" gaps.

- **Complexity:** Use `gocyclo -top N hunted.go` for analysis.
  Threshold is 2× average CC. Flat switches (each case is a single
  call/assignment) are acceptable above threshold.

## Code style

- **`cond[T]` ternary:** Use `cond(b, t, f)` liberally to replace
  `x := a; if c { x = b }`. It's eager — never use with
  nil-guarded calls or bounds-guarded indexing.

- **DRY helpers:** Prefer one parameterized helper over N
  near-identical variants, but balance DRY with clarity — too many
  parameters (especially abstract pointer-swapping or axis-generic
  arguments) hurts readability. When a helper needs >5 parameters
  to stay generic, consider inlining with explicit per-case code.
  Helpers should target CC ≤ half the project average.

- **Behavior dispatch uses two patterns.** Function pointers
  (Controller, PostMove, PreAct, FrameFunc) for substantial logic
  (>5 lines), dynamic behavior (set/cleared at runtime), or
  per-instance variation. Kind switches for small, contrastive
  reactions where seeing all kinds side-by-side aids comprehension
  (Crunch scoring, Reap, MobCollide).

- **No backup files** in the repo.

## Unit testing

- **Clarity over DRY in tests.** Each test should be self-contained
  and obvious. Copious comments explain what is being tested, why,
  and how it maps to game behavior. Prefer readability over
  eliminating duplication.

- **Coverage targets.** Aim for 100% statement coverage (`go tool
  cover`) and 100% condition coverage (`gobco`). When adding or
  modifying production code, update tests to maintain both.

- **Test without raylib.** Keep `SoundsLoaded=false` in test Game
  structs so PlaySfx/PlayExplosion skip raylib audio calls. Seed
  `g.Rng` when functions use randomness (OpenCrate, MakeSparks).

- **Mock external dependencies with gomonkey/v2.** For functions
  that call methods outside the file under test (e.g. MobCollide →
  OpenCrate, Crunch, Explode), mock them with
  `gomonkey.ApplyMethodFunc` to test the caller's branching in
  isolation. This eliminates setup for downstream state (testDeadDef,
  HeartCount, Threat, etc.) and makes tests immune to changes in
  the mocked functions' internals. Tests must run with
  `-gcflags=all=-l` (disable inlining).

- **Minimal test helpers.** `newTestGame()` builds a Game with
  seeded RNG and wall tile config. `newObj(x, y)` builds an 8×8
  GameObject. Use raw `&Game{}` or `&GameObject{}` when only a few
  fields matter (especially with mocks).

- **Test naming.** Subtests use descriptive CamelCase names that
  describe the scenario (e.g. `FriendlyOpensCrate`,
  `BothBlockedNoSlide`, `DiagonalSkipped`).

## Architecture notes

- **config.yaml is the single source of truth** for sprites, tiles,
  assets, sounds, and the thug roster. It replaced both hardcoded
  Go constants and the old map.yaml.

- **Thugs are references, not definitions.** Thug sprite data lives
  in the `sprites:` section (with `kind: thug`), and the `thugs:`
  list is just an ordered list of sprite names. Each thug is
  followed by its dead counterpart in the sprites section.

- **Sprite `final` chains** resolve at load time by name lookup in
  `g.Defs` (e.g. bomb → boom).

- **Kind is a bit-flag type** (`1 << iota`), not a plain enum.
  `Friendly = Player | Waif` is a composite mask used for collision
  checks.

- **The Game struct uses embedded sub-structs** (TileConfig,
  SoundState, etc.) to organize fields without pointer indirection.

- **Named PICO-8 color vars** (`PicoBlack`..`PicoPeach`) plus
  overlay/highlight colors replace all magic `rl.Color{}` literals
  and array indices.

- **Balance params** (`heart_count`, `bomb_count`, `mine_count`,
  `threat_max_start`) live in `config.yaml` under `config:`, loaded
  into `GameConfig` and copied to `Game` at game start.

- **Prefer config.yaml over hardcoded constants.** Any value that
  could feasibly be tuned without recompiling—counts, speeds,
  timings, thresholds, colors, asset paths—should live in
  config.yaml rather than as a Go literal.
