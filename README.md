# moleman (Go/YAML prototype)

moleman is a minimal, shell-first workflow runner for automating advanced local AI coding
workflows. It executes a YAML plan made of reusable steps, groups, and loops, and writes
artifacts per run for review and auditing.

> Status: early prototype. Expect breaking config and CLI changes.

## Install (local)

```
go build -o moleman
./moleman --help
```

## Quick Start (AI coding)

```
./moleman init
./moleman pipelines
./moleman run --pipeline default --prompt "Fix the lint errors in src/"
```

## Makefile targets

```
make fmt
make test
make vet
make lint
make build
make check
```

## Config (`moleman.yaml`)

```yaml
version: 1

steps:
  code:
    type: run
    # Replace with your local AI CLI command
    run: "ai-code --non-interactive --prompt {{ shellEscape (printf \"%s\\n\\nDo not offer next steps or additional tasks; just return the requested output.\" .input.prompt) }} --context {{ shellEscape .git.diff }}"
    timeout: 30m
    capture: [stdout, stderr, exitCode]

  lint:
    type: run
    run: "pnpm lint"
    timeout: 10m

groups:
  default:
    - type: ref
      id: code
    - type: ref
      id: lint

pipelines:
  default:
    plan:
      - type: ref
        id: default
```

## Config lookup

When `--config` is not provided, moleman searches in this order:

1. `./moleman.yaml`
2. `./.moleman/configs/default.yaml`
3. `~/.moleman/configs/default.yaml`

The `.moleman/configs/` path is a good place for personal configs that you
do not want checked into the repo (and it is ignored by `.gitignore`).

## Config Reference (v1)

Top-level:

- `version` (number, required)
- `steps` (map, optional)
- `groups` (map, optional)
- `pipelines` (map, required)

Step (`type: run`):

- `run` (string, required)
- `stdin` (string, optional)
- `stdinFile` (string, optional)
- `env` (map, optional)
- `timeout` (string duration, optional)
- `capture` (list, optional: `stdout`, `stderr`, `exitCode`)
- `print` (list, optional: `stdout`, `stderr`)
- `parse` (optional: `kind: json`, `into: <key>`)
- `shell` (string, optional)

Node types:

- `run` (inline run node, same fields as step)
- `ref` (`id` pointing to a step or group)
- `group` (`body` array of nodes)
- `loop` (`maxIters`, `until`, `body`)

## Node Types

- `run`: execute a shell command (directly or via a step reference)
- `ref`: reference a step or group by id
- `loop`: repeat a body until a condition is true

## Loop Conditions

Loop `until` uses a Go-style expression evaluated against the runtime context.
Examples:

```
steps.lint.exitCode == 0
stepsHistory.lint[0].exitCode != 0
git.branch == "main"
```

`{{ ... }}` wrappers are accepted but optional.

## Runtime Context (templating + conditions)

- `input.prompt`
- `git.diff`, `git.status`, `git.branch`, `git.root`
- `steps.<id>.stdout`, `steps.<id>.stderr`, `steps.<id>.exitCode`
- `stepsHistory.<id>[i].stdout`, `.stderr`, `.exitCode`
- `vars.*` (populated by `parse`)

Template helper functions:

- `shellEscape` (wraps a value for safe use as a POSIX shell argument)

## Artifacts

Each run creates:

```
.moleman/runs/<timestamp>-<pipeline>/
  input.md
  resolved-plan.json
  steps/<step-id>/<iter>/stdout.log
  steps/<step-id>/<iter>/stderr.log
  steps/<step-id>/<iter>/meta.json
  diffs/
  summary.md
```

## Examples

See `examples/` for minimal and looped AI workflows.

## Etymology

This project is named after Hans Moleman, the eternally unlucky Springfield everyman who somehow keeps coming back no matter how many times the universe
drops a piano on him. `moleman` might get knocked around by failed runs, exploding diffs, and wild reviews, yet
is ready to resurface with a fresh go at the workflow. If Moleman can survive 44 on‑screen deaths, `moleman` can
survive a few yolo blunders and keep going.
