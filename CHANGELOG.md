# Changelog

## Unreleased

- Ensure JSON outputs are accessible via `structured_output` in conditions.
- Disable Codex resume when output schema/file is used.
- Stream agent stdout/stderr when `--verbose` is enabled.
- Fix review schema required fields and show stderr in node failures.

## 0.1.1

- Ship shared `agents.yaml` defaults and agent `extends` overrides.
- Add Codex `outputSchema`/`outputFile` support plus a review schema template.
- Preflight agent commands and schema paths with clearer failure messages.
- Treat missing outputs as false in loop condition evaluation.
- Refresh README and examples for shared agent defaults and Codex schemas.

## 0.1.0

- Initial prototype of the moleman CLI runner.
