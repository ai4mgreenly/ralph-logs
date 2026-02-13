# ralph-shows

Lightweight dashboard for the Ralph pipeline. Displays the current state of goals managed by ralph-plans — what's running, queued, done, cancelled, and in draft. A static Preact SPA that polls the ralph-plans API.

Philosophy: deliberately minimalist. Preact + Signals, no CSS framework, no build toolchain beyond Deno.

## Architecture

Part of a multi-service system:

| Service | Language | Port | Purpose |
|---------|----------|------|---------|
| **ralph-plans** | Go + SQLite | 5001 | Goal storage and state machine |
| **ralph-shows** | Deno + Preact | 5000 | This project — Web UI dashboard |
| **ralph-runs** | Ruby | 5002 | Orchestrator + agent loop |
| **ralph-logs** | Go | 5003 | Real-time log streaming |
| **ralph-counts** | Python | 5004 | Metrics dashboard |

### How It Works

The dashboard is a static SPA that polls `GET /goals?status=<status>` on ralph-plans every 5 seconds. No auth — both services are localhost-only.

### Source Layout

```
src/
├── main.tsx          # Mount Preact app into #app
├── app.tsx           # Top-level App component, starts polling
├── api.ts            # Fetch wrapper for ralph-plans API
├── state.ts          # Preact Signals for goals + polling logic
└── components/
    ├── running.tsx    # Running goals section
    ├── queued.tsx     # Queued goals section
    ├── done.tsx       # Done goals section
    ├── cancelled.tsx  # Cancelled goals section
    ├── draft.tsx      # Draft goals section
    └── goal-detail.tsx # Goal detail view
```

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
| `goal-spot-check` | Spot-check a goal |

## Development

### Tech Stack

- **Deno** runtime + TypeScript
- **Preact** with JSX (`react-jsx` transform)
- **@preact/signals** for reactive state
- Static SPA bundled via `build.ts`, dev server via `dev.ts`

### Commands

```sh
deno task dev    # Bundle + serve with watch
deno task build  # Production bundle
```

Or just `./launch.sh` which runs `deno task dev`.

### Version Control

This project uses **git**.

### Code Style

- TypeScript throughout, Deno idioms
- Preact functional components with Signals
- Minimal inline CSS — dark theme, monospace font
- Goal scripts are Ruby, return JSON: `{"ok": true/false, ...}`
- Minimalist — no abstractions for one-time operations

### Environment

Configured via `.envrc` (direnv). `PATH` includes `scripts/bin/` for direct script access. Services communicate via `RALPH_*_HOST/PORT` env vars.

## Directory Structure

```
ralph-shows/
├── src/                              # Preact SPA source
│   ├── main.tsx                      # Entry point
│   ├── app.tsx                       # App component
│   ├── api.ts                        # API client
│   ├── state.ts                      # Signals + polling
│   └── components/                   # UI components
├── scripts/
│   ├── bin/                          # Symlinks to goal scripts (on PATH)
│   └── goal-*/run                    # Goal state management scripts
├── dist/                             # Built output
├── index.html                        # HTML shell
├── build.ts                          # Build script
├── dev.ts                            # Dev server
├── deno.json                         # Tasks, imports, compiler options
├── launch.sh                         # Entry point: runs deno task dev
├── .claude/
│   ├── library/                      # Skills (modular instruction sets)
│   └── skillsets/                    # Composite skill bundles
├── .envrc                            # direnv config
└── AGENTS.md                         # This file
```

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

- `meta` — For improving the .claude/ system (preloads: jj, pipeline, goal-authoring, align)

### For Ralph

When Ralph executes a goal in this repo, it receives only `AGENTS.md` as project context. This file is responsible for getting Ralph everything it needs.

## Goal Authoring

Goals are markdown files with required sections: `## Objective`, `## Reference`, `## Outcomes`, `## Acceptance`.

Key principles: specify WHAT not HOW, reference liberally, make discovery explicit, include measurable acceptance criteria, trust Ralph to iterate.

Full guide: `.claude/library/goal-authoring/SKILL.md`

## Common Tasks

**Adding a component:** Create `src/components/<name>.tsx`, import in `app.tsx`.

**Adding a new goal status view:** Create component in `src/components/`, add a signal in `state.ts`, update `api.ts` if needed, wire into `app.tsx`.

**Changing API base URL:** Edit `src/api.ts`.

**Adding a goal command:** Create `scripts/<name>/run` (Ruby, returns JSON), symlink from `scripts/bin/<name>`.

**Adding a skill:** Create `.claude/library/<name>/SKILL.md` with YAML frontmatter (name, description). Add to relevant skillset JSON.
