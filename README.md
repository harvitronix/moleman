# moleman

`moleman` is a local CLI for running multi-step AI workflows from YAML.

I use it for things like write -> review -> fix loops across Codex/Claude without custom shell scripts.

It does not include any model runtime. It just executes CLIs you already have installed.

## Install

Requirements:

- Node.js 24+
- `pnpm`
- Agent CLI on `PATH` (`codex`, `claude`, or a custom command)

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

Create starter config files:

```bash
moleman init
```

Run with a prompt:

```bash
moleman run --prompt "Fix lint errors in src"
```

Check setup:

```bash
moleman doctor
```

Inspect run output:

```bash
ls .moleman/runs/
```

## Commands

```bash
moleman run --prompt "..." [--config ./moleman.yaml]
moleman init [--config ./moleman.yaml] [--force]
moleman doctor [--config ./moleman.yaml]
moleman agents [--config ./moleman.yaml]
moleman explain [--config ./moleman.yaml]
moleman version
```

Common `run` flags: `--prompt-file`, `--workdir`, `--dry-run`, `--verbose`.

If `--config` is omitted, lookup order is:

1. `./moleman.yaml`
2. `./.moleman/configs/default.yaml`
3. `~/.moleman/configs/default.yaml`

`agents.yaml` must be in the same directory as the selected config file.

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

## Example 2: Write/Review Loop

```yaml
version: 1

agents:
  claude_review:
    extends: claude
    args: ["--output-format", "json"]

workflow:
  - type: agent
    name: write
    agent: codex
    input:
      from: input
    output:
      toNext: true

  - type: loop
    maxIters: 3
    until: "outputs.review_json.structured_output.must_fix_count == 0"
    body:
      - type: agent
        name: review
        agent: claude_review
        input:
          prompt: "Review current diff and list must-fix issues."
        output:
          toNext: true
      - type: agent
        name: fix
        agent: codex
        session:
          resume: last
        input:
          from: review
        output:
          toNext: true
```

## Notes

- Supported agent types: `codex`, `claude`, `generic`
- Template fields: `.input`, `.outputs`, `.last`, `.sessions`
- `loop.until` is evaluated as JavaScript against workflow data

## Dev

```bash
pnpm build
pnpm typecheck
pnpm test
pnpm check
```
