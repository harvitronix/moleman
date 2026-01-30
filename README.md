# moleman

moleman is a minimal, shell-first workflow runner for agent-to-agent AI coding
workflows. It executes a YAML workflow of agent nodes, moves data between
agents, and writes artifacts per run for review and auditing.

> Status: early prototype. Expect breaking config and CLI changes.

## Motivation

I often bounce between sessions that write code and sessions that review it.
Especially in Ralph loops, the write -> review -> write orchestration gets
tedious. moleman makes it easy to define and reuse those loops across tools,
while keeping the workflow explicit and repeatable.

## Etymology

This project is inspired by the resilience of Hans Moleman, the eternally
unlucky Springfield everyman who somehow keeps coming back no matter how many
times the universe drops a piano on him. moleman gets knocked around by failed
runs, exploding diffs, and wild prompts, yet always resurfaces with a fresh set
of artifacts, a clear status, and another go at the workflow. If he can survive
44 on-screen deaths, our pipeline can survive a few bad edits and keep digging.
[Hans Moleman profile](https://simpsons.fandom.com/wiki/Hans_Moleman)

## Install (from repo)

```
go build -o moleman
./moleman --help
```

## Quick Start (modify this repo)

```
./moleman run --prompt "Fix the lint errors in src/"
```

Useful commands:

- `moleman run` - execute the workflow
- `moleman agents` - list configured agents
- `moleman explain` - print the resolved workflow

## Makefile targets

```
make fmt
make test
make vet
make lint
make build
make check
```

## Example Config (`moleman.yaml`)

```yaml
version: 1

agents:
  codex:
    type: codex
    args: ["--full-auto"]
    timeout: 30m
    capture: [stdout, stderr, exitCode]

workflow:
  - type: agent
    name: write
    agent: codex
    input:
      from: input
    output:
      stdout: true
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
- `agents` (map, required)
- `workflow` (list, required)

Agent config:

- `type` (string, required: `codex`, `claude`, `generic`)
- `command` (string, required for `generic`, optional otherwise)
- `args` (list, optional)
- `env` (map, optional)
- `timeout` (string duration, optional)
- `capture` (list, optional: `stdout`, `stderr`, `exitCode`)
- `print` (list, optional: `stdout`, `stderr`)
- `session` (optional: `{resume: "last" | "new"}`)

Workflow node (type `agent`):

- `name` (string, required, unique in workflow)
- `agent` (string, required: key in `agents`)
- `input` (one of `prompt`, `file`, `from`)
- `output` (one of `toNext`, `file`, `stdout`; choose exactly one)
- `session` (optional: `{resume: "last" | "new"}`)

Workflow node (type `loop`):

- `maxIters` (number, required)
- `until` (string, required; expression)
- `body` (list of workflow nodes)

## Templates and data

Templates use Go text/template syntax. Available data:

- `.input.prompt` (top-level input prompt)
- `.outputs` (map of outputs by node name; JSON is stored as `<name>_json`)
- `.last` (last output passed to next)
- `.sessions` (agent session IDs when available)

## Sessions

- Codex: `session.resume: last` maps to `codex exec resume --last`.
- Claude: `session.resume: last` uses the latest `session_id` parsed from
  Claude JSON output (`--output-format json`).

## Artifacts

Each run creates:

```
.moleman/runs/<timestamp>-workflow/
  input.md
  resolved-workflow.json
  nodes/<node-name>/stdout.log
  nodes/<node-name>/stderr.log
  nodes/<node-name>/meta.json
  diffs/
  summary.md
```

## Examples

See `examples/` for minimal and looped AI workflows.
For a Codex + Claude review loop, check `examples/codex-claude-loop.yaml`.

## Versioning

`moleman --version` prints the current version. The version is tracked in
`version.go` and is intended to be bumped via release automation.
