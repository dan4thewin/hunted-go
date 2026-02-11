Use short, concise commit messages.

## Architecture notes

- **config.yaml is the single source of truth** for sprites, tiles, assets, sounds, and the thug roster. It replaced both hardcoded Go constants and the old map.yaml.

- **Thugs are references, not definitions.** Thug sprite data lives in the `sprites:` section (with `kind: thug`), and the `thugs:` list is just an ordered list of sprite names. Each thug is followed by its dead counterpart in the sprites section.

- **Sprite `final` chains** resolve at load time by name lookup in `g.Defs`. Some chains are multi-hop (e.g. bomb → livebomb → boom).

- **Kind is a bit-flag type** (`1 << iota`), not a plain enum. `Friendly = Player | Waif` is a composite mask used for collision checks.

- **The Game struct uses embedded sub-structs** (TileConfig, SoundState, etc.) to organize fields without pointer indirection.
