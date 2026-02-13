**Surface contradictions immediately.** If stored facts, code comments, documentation, or architecture notes conflict with actual code behavior, flag the contradiction to the user at high priority before proceeding. Do not silently trust one source over another—the user decides which is correct.

Use short, concise commit messages.

As the codebase evolves, frequently suggest brief updates to the architecture notes below so they stay current. Propose changes early and often—small, incremental updates are easier to review than large rewrites.

## Architecture notes

- **config.yaml is the single source of truth** for sprites, tiles, assets, sounds, and the thug roster. It replaced both hardcoded Go constants and the old map.yaml.

- **Thugs are references, not definitions.** Thug sprite data lives in the `sprites:` section (with `kind: thug`), and the `thugs:` list is just an ordered list of sprite names. Each thug is followed by its dead counterpart in the sprites section.

- **Sprite `final` chains** resolve at load time by name lookup in `g.Defs`. Some chains are multi-hop (e.g. bomb → livebomb → boom).

- **Kind is a bit-flag type** (`1 << iota`), not a plain enum. `Friendly = Player | Waif` is a composite mask used for collision checks.

- **The Game struct uses embedded sub-structs** (TileConfig, SoundState, etc.) to organize fields without pointer indirection.

- **Named PICO-8 color vars** (`PicoBlack`..`PicoPeach`) plus overlay/highlight colors replace all magic `rl.Color{}` literals and array indices.

- **Balance params** (`heart_count`, `bomb_count`, `mine_count`, `threat_max_start`) live in `config.yaml` under `config:`, loaded into `GameConfig` and copied to `Game` at game start.

- **Prefer config.yaml over hardcoded constants.** Any value that could feasibly be tuned without recompiling—counts, speeds, timings, thresholds, colors, asset paths—should live in config.yaml rather than as a Go literal.
