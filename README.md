# moleman

`moleman` is a local CLI for running multi-step AI workflows from YAML workflow files.

You define steps in a workflow file (`moleman.yaml`), then run that workflow with a starter prompt:

- `--prompt "..."` for inline input
- `--prompt-file ./prompt.md` for file input

`moleman` executes each node in order, passes outputs between nodes, and writes run artifacts under `.moleman/runs/`.

It does not provide models. It runs CLIs you already have installed.

### Architecture: Runtime vs Profile vs Node

- **Workflow**: an ordered graph of nodes (`agent` and `loop`) executed top-to-bottom from a starter prompt.
- **Agent Runtime**: built-in support in code (`codex`, `claude`, `generic`).
- **Agent Profile**: YAML config for a runtime (usually in `agents.yaml`), e.g. model/args/timeout/session defaults.
- **Agent Node**: a `type: agent` step inside `workflow:` that references an agent profile by name.

So yes, runtime support is code-level, while YAML defines profiles and workflow nodes that use those profiles.
If you want to support a new runtime (for example `ampcode`), that requires code changes.

Minimal profile + node wiring:

```yaml
agents:
  reviewer:
    type: claude
    args: ["--output-format", "json"]

workflow:
  - type: agent
    name: review
    agent: reviewer
    input:
      from: input
    output:
      toNext: true
```

Typical workflow shapes:

- write -> review -> fix
- build -> test -> review <-> build (loop until clean)
- spec -> implement -> test -> summarize

## Install

Requirements:

- Node.js 24+
- `pnpm`
- agent CLI on `PATH` (`codex`, `claude`, or a custom command)

Global install:

```bash
pnpm add -g moleman
moleman --help
```

From this repo:

```bash
corepack enable
pnpm install
pnpm build
pnpm link --global
moleman --help
```

## Basic Use

First, create or choose a workflow file.

Start from one of the files in `examples/`, then edit it for your repo.

Run the workflow with a starter prompt:

```bash
moleman run --prompt "Fix lint errors in src"
```

Or from a prompt file:

```bash
moleman run --prompt-file ./prompt.md
```

Validate workflow + environment:

```bash
moleman doctor
```

Inspect run output:

```bash
ls .moleman/runs/
```

## Commands

```bash
moleman run --prompt "..." [--workflow ./moleman.yaml]
moleman doctor [--workflow ./moleman.yaml]
moleman agents [--workflow ./moleman.yaml]
moleman explain [--workflow ./moleman.yaml]
moleman version
```

Common `run` flags: `--prompt-file`, `--workdir`, `--dry-run`, `--verbose`.

If `--workflow` is omitted, lookup order is:

1. `./moleman.yaml`
2. `./.moleman/workflows/default.yaml`
3. `~/.moleman/workflows/default.yaml`

`agents.yaml` must be in the same directory as the selected workflow file.

## Example 1: Minimal

`agents.yaml`

```yaml
agents:
  codex:
    type: codex
    args: ["--full-auto"]
    timeout: 45m
```

`moleman.yaml`

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

Run:

```bash
moleman run --prompt "Refactor src/session.ts to simplify error handling"
```

## Example 2: Build/Test/Review Loop

```yaml
version: 1

agents:
  reviewer:
    extends: claude
    args: ["--output-format", "json"]

workflow:
  - type: agent
    name: build
    agent: codex
    input:
      prompt: "Run build and fix compile errors until it succeeds."
    output:
      toNext: true

  - type: loop
    maxIters: 3
    until: "outputs.review_json.structured_output.must_fix_count == 0"
    body:
      - type: agent
        name: test
        agent: codex
        session:
          resume: last
        input:
          prompt: "Run tests and fix failures."
        output:
          toNext: true
      - type: agent
        name: review
        agent: reviewer
        input:
          prompt: "Review current diff. Return must-fix count in JSON."
        output:
          toNext: true
```

## Notes

- Built-in agent runtimes: `codex`, `claude`, `generic`
- Template fields: `.input`, `.outputs`, `.last`, `.sessions`
- `loop.until` is evaluated as JavaScript against workflow data

## Dev

```bash
pnpm build
pnpm typecheck
pnpm test
pnpm check
```
