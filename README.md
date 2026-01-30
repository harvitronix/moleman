# moleman

moleman is a minimal, shell-first workflow runner for agent-to-agent AI coding
workflows. It executes a YAML workflow of agent nodes, moves data between
agents, and writes artifacts per run for review and auditing.

> Status: early prototype. Expect breaking config and CLI changes.

## Contents

- [At a glance](#at-a-glance)
- [Motivation](#motivation)
- [Etymology](#etymology)
- [Requirements](#requirements)
- [Install](#install)
- [Quick Start](#quick-start)
- [Core concepts](#core-concepts)
- [How moleman runs a workflow](#how-moleman-runs-a-workflow)
- [CLI usage](#cli-usage)
- [Makefile targets](#makefile-targets)
- [Configuration](#configuration)
- [Templates and data](#templates-and-data)
- [Sessions](#sessions)
- [Artifacts](#artifacts)
- [Examples](#examples)
- [FAQ](#faq)
- [Versioning](#versioning)
- [Troubleshooting](#troubleshooting)

## At a glance

- Run repeatable, multi-agent workflows from a single YAML file.
- Pass outputs between steps without manual copy/paste.
- Keep a full trail of artifacts for review, diffs, and debugging.
- Use Codex, Claude, or any generic CLI command.
- Works locally with your existing CLIs—no hosted service required.

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

## Requirements

- Go 1.21+ (for `go install` / `go build`)
- A configured agent CLI (Codex, Claude, or any generic command)

## Install

From the repo:

```
go build -o moleman
./moleman --help
```

From a Git checkout (installs into `GOBIN`/`GOPATH/bin`):

```
go install ./...
```

## What you get

- A single binary that runs YAML-defined agent workflows.
- A predictable run directory with inputs, outputs, logs, diffs, and summaries.
- A way to chain agents together (e.g., write -> review -> fix) without glue scripts.

## Quick Start

1) Create a config file (minimal example). Agent defaults live in
`agents.yaml`, so you only need the workflow here:

```yaml
# moleman.yaml
version: 1

workflow:
  - type: agent
    name: write
    agent: codex
    input:
      from: input
    output:
      stdout: true
```

2) Run a prompt against the workflow:

```
./moleman run --prompt "Fix the lint errors in src/"
```

3) Inspect artifacts:

```
ls .moleman/runs/
```

4) If you keep configs outside the repo, pass `--config`:

```
./moleman run --config ~/.moleman/configs/default.yaml --prompt "..."
```

Useful commands:

- `moleman run` - execute the workflow
- `moleman agents` - list configured agents
- `moleman explain` - print the resolved workflow

## Supported agents

- `codex` - OpenAI Codex CLI (recommended for code edit loops).
- `claude` - Anthropic Claude CLI (great for review-style steps).
- `generic` - any shell command that reads stdin and writes stdout.

## Core concepts

- Agent: a configured CLI runner (Codex, Claude, or a generic command).
- Workflow: an ordered list of nodes (agent steps or loops).
- Input/output: each node consumes input and produces one output for the next
  step or a file.
- Artifacts: each run writes a full trail of prompts, outputs, diffs, and logs
  under `.moleman/runs/`.

## How moleman runs a workflow

1) Resolve config from `--config` or the default lookup paths.
2) Render templates using the available data (`.input`, `.outputs`, `.last`).
3) Execute each node in order, optionally looping.
4) Write artifacts for every node and a summary for the run.

## CLI usage

```
moleman run --prompt "..." [--config path/to/moleman.yaml]
moleman agents [--config ...]
moleman explain [--config ...]
moleman --version
```

Common flags:

- `--prompt` - top-level prompt passed to the workflow.
- `--config` - path to `moleman.yaml` (optional if you use default locations).

## Makefile targets

```
make fmt
make test
make vet
make lint
make build
make check
```

## Configuration

### Example Config (`moleman.yaml`)

```yaml
version: 1

workflow:
  - type: agent
    name: write
    agent: codex
    input:
      from: input
    output:
      stdout: true
```

### Shared agent defaults (`agents.yaml`)

Define shared agent presets once per repo so workflows stay small:

```yaml
# agents.yaml
agents:
  codex:
    type: codex
    args: ["--full-auto"]
    timeout: 45m
    capture: [stdout, stderr, exitCode]
```

Override defaults in `moleman.yaml` when needed:

```yaml
agents:
  codex_review:
    extends: codex
    args: ["--full-auto"] # add model/thinking flags for your CLI here
```

### Example loop (write -> review -> write)

```yaml
version: 1

workflow:
  - type: loop
    maxIters: 3
    until: 'contains(.last, "LGTM")'
    body:
      - type: agent
        name: write
        agent: codex
        input:
          from: input
        output:
          toNext: true
      - type: agent
        name: review
        agent: claude
        input:
          from: last
        output:
          toNext: true
```

### Config lookup

When `--config` is not provided, moleman searches in this order:

1. `./moleman.yaml`
2. `./.moleman/configs/default.yaml`
3. `~/.moleman/configs/default.yaml`

The `.moleman/configs/` path is a good place for personal configs that you
do not want checked into the repo (and it is ignored by `.gitignore`).

`agents.yaml` is loaded from the same directory as the resolved config file and
is required. The repo ships a default `agents.yaml` you can edit or extend.

### Config reference (v1)

Top-level:

- `version` (number, required)
- `agents` (map, optional; overrides or extends `agents.yaml`)
- `workflow` (list, required)

Agent config:

- `extends` (string, optional; name of an agent in `agents.yaml`)
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

### Inputs and outputs

Each agent node must specify exactly one input and exactly one output.

Inputs:

- `input.prompt` - inline prompt string.
- `input.file` - load prompt from a file.
- `input.from` - pull from `input` or `last` (previous output).

Outputs:

- `output.toNext` - pass output to the next node.
- `output.file` - write output to a file.
- `output.stdout` - write to stdout (useful for simple workflows).

### Tips

- For generic agents, set `command` to the CLI path and `args` for defaults.
- Use `output.toNext: true` to pass output along in multi-step workflows.
- Use `output.file` for long outputs you want to diff or re-ingest later.
- Keep personal configs in `.moleman/configs/` to avoid committing secrets.

## Templates and data

Templates use Go text/template syntax. Available data:

- `.input.prompt` (top-level input prompt)
- `.outputs` (map of outputs by node name; JSON is stored as `<name>_json`)
- `.last` (last output passed to next)
- `.sessions` (agent session IDs when available)

Template snippet example:

```yaml
input:
  prompt: |
    Apply fixes based on the review:
    {{ .outputs.review }}
```

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

Artifacts are grouped per node so you can inspect or diff exactly what happened
at each step. The `summary.md` includes a high-level view of the run.

## Examples

See `examples/` for minimal and looped AI workflows.
For a Codex + Claude review loop, check `examples/codex-claude-loop.yaml`.

## FAQ

- Do I need an agent installed? Yes. moleman just orchestrates; it does not
  bundle an agent.
- Where do I look when a run fails? Start in `.moleman/runs/<timestamp>-workflow/`.
- Can I keep prompts out of git? Yes. Put configs in `.moleman/configs/` or pass
  `--config` to point at a private file.
- Can I use any CLI? Yes. Use `type: generic` and point `command` at the binary.
- Does moleman modify my repo? Only if your agent does. moleman itself only
  writes to `.moleman/runs/` and any explicit `output.file` paths.

## Versioning

`moleman --version` prints the current version. The version is tracked in
`version.go` and is intended to be bumped via release automation.

## Troubleshooting

- No agents listed: make sure `moleman.yaml` exists or pass `--config`.
- Agent command fails: verify the agent CLI is installed and on `PATH`.
- Missing outputs: check `.moleman/runs/<timestamp>-workflow/nodes/<name>/`.
 - Weird template output: confirm you used the right data (`.input`, `.last`,
   `.outputs`).
