# Changelog

## Unreleased

- Rewrite CLI implementation from Go to TypeScript for Node.js 24+.
- Switch packaging/distribution to pnpm (`moleman` bin via `pnpm add -g`).
- Port workflow execution, workflow validation, templating, loop conditions, and artifacts to TS.
- Replace Go CI/release workflow with Node-based check/build and release-please settings.
- Tighten README to a short, practical guide with focused examples.
- Upgrade pnpm to `10.30.1`, refresh lockfile, and bump dependencies to latest available versions.
- Clarify README intro with workflow/prompt execution model and concrete workflow patterns.
- Rename CLI and docs terminology from `config` to `workflow` (`--workflow`, `.moleman/workflows/`).

## 0.1.1

- Ship shared `agents.yaml` defaults and agent `extends` overrides.
- Add Codex `outputSchema`/`outputFile` support plus a review schema template.
- Preflight agent commands and schema paths with clearer failure messages.
- Treat missing outputs as false in loop condition evaluation.
- Refresh README and examples for shared agent defaults and Codex schemas.

## 0.1.0

- Initial prototype of the moleman CLI runner.
