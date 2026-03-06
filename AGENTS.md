# ralph-logs

Real-time log streaming dashboard for the Ralph pipeline. Tails log files across the Ralph system and broadcasts updates to browser clients over WebSocket. A single Go binary — no framework, no dependencies beyond gorilla/websocket.

Philosophy: deliberately minimalist. Embed the HTML, tail the files, broadcast the bytes.

## Workflow

All code changes in this repository go through goals — never direct edits. When something needs changing, create a goal and let the pipeline execute it.

The only exception: the user gives extremely explicit instructions to make changes directly. "Extremely explicit" means the user unambiguously requests a direct change — e.g. "edit this file now", "make this change locally", "don't create a goal". Ambiguity defaults to creating a goal.

If you're unsure whether to make a direct change or create a goal, create a goal.

## Architecture

Part of a multi-service system:

| Service | Language | Port | Purpose |
|---------|----------|------|---------|
| **ralph-plans** | Go + SQLite | 5001 | Goal storage and state machine |
| **ralph-shows** | Deno + Preact | 5000 | Web UI dashboard |
| **ralph-runs** | Ruby | 5002 | Orchestrator + agent loop |
| **ralph-logs** | Go | 5003 | This project — Real-time log streaming |
| **ralph-counts** | Python | 5004 | Metrics dashboard |

### How It Works

The server accepts glob patterns as CLI args, discovers matching log files, and tails them. A registry rescans every 2 seconds for new/removed files. Each tailer detects inode changes (log rotation) and seamlessly switches to the new file. Browser clients connect via WebSocket and receive live updates.

### Source Layout

```
ralph-logs/
├── main.go              # Everything: tailer, broker, registry, HTTP server
├── index.html           # Browser UI (embedded, dark theme, monospace)
├── favicon.svg          # Embedded favicon
├── go.mod               # Single dependency: gorilla/websocket
├── Makefile             # Build and run targets
├── launch.sh            # Entry point for production
├── scripts/
│   ├── bin/             # Symlinks to goal scripts (on PATH)
│   └── goal-*/run       # Goal state management scripts
├── .claude/
│   ├── library/         # Skills (modular instruction sets)
│   └── skillsets/       # Composite skill bundles
├── .envrc               # direnv config
└── AGENTS.md            # This file
```

### WebSocket Protocol

Messages are JSON with a `type` field:

| Type | Direction | Purpose |
|------|-----------|---------|
| `init` | server → client | Initial state: paths list + selected file content |
| `append` | server → client | New data appended to a file |
| `reset` | server → client | File rotated (inode changed), buffer cleared |
| `paths` | server → client | Watched file list changed |
| `select` | client → server | Client requests content of a specific file |

### Goal Scripts

Goal management scripts live in `scripts/goal-*/run` (Ruby, return JSON). Symlinked from `scripts/bin/` and available on PATH via `.envrc`.

| Script | Purpose |
|--------|---------|
| `goal-list` | List goals by status |
| `goal-get` | Get a single goal |
| `goal-create` | Create a new goal (draft) |
| `goal-queue` | Queue a draft goal |
| `goal-start` | Mark a goal as running |
| `goal-done` | Mark a goal as done |
| `goal-stuck` | Mark a goal as stuck |
| `goal-retry` | Retry a stuck goal |
| `goal-cancel` | Cancel a goal |
| `goal-comment` | Add a comment to a goal |
| `goal-comments` | List comments on a goal |

## Development

### Tech Stack

- **Go** (standard library + gorilla/websocket)
- Single binary, no external config
- HTML/JS embedded via `go:embed`

### Commands

```sh
make              # Build the binary
make run          # Build and run with default log patterns
./launch.sh       # Production entry point
```

### Version Control

This project uses **git**.

### Code Style

- Go standard library idioms, minimal abstraction
- Single-file server (`main.go`)
- Goal scripts are Ruby, return JSON: `{"ok": true/false, ...}`
- Minimalist — no abstractions for one-time operations

### Environment

Configured via `.envrc` (direnv). `PATH` includes `scripts/bin/` for direct script access. Services communicate via `RALPH_*_HOST/PORT` env vars.

## Skills

Skills are modular instruction sets in `.claude/library/<name>/SKILL.md`.

- **Load a skill**: `/load <name>` reads the skill into context
- **Load multiple**: `/load name1 name2`

### Skillsets

Composite bundles in `.claude/skillsets/<name>.json`:

```json
{
  "preload": ["skill-a"],
  "advertise": [{"skill": "skill-b", "description": "When to use"}]
}
```

- `preload` — loaded immediately when skillset is activated
- `advertise` — shown as available, loaded on demand with `/load`

Available skillsets:

- `meta` — For improving the .claude/ system (preloads: jj, align)

### For Ralph

When Ralph executes a goal in this repo, it receives only `AGENTS.md` as project context. This file is responsible for getting Ralph everything it needs.

## Common Tasks

**Changing watched log patterns:** Edit `launch.sh` or `Makefile` — patterns are CLI args to the binary.

**Modifying the browser UI:** Edit `index.html`, rebuild with `make`.

**Adding a goal command:** Create `scripts/<name>/run` (Ruby, returns JSON), symlink from `scripts/bin/<name>`.

**Adding a skill:** Create `.claude/library/<name>/SKILL.md` with YAML frontmatter (name, description). Add to relevant skillset JSON.
