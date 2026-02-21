import test from "node:test";
import assert from "node:assert/strict";
import { promises as fs } from "node:fs";
import os from "node:os";
import path from "node:path";
import { loadWorkflow } from "../src/workflow.js";
import { run } from "../src/run.js";

async function tempDir(prefix: string): Promise<string> {
  return fs.mkdtemp(path.join(os.tmpdir(), prefix));
}

test("run executes steps and writes artifacts", async () => {
  const dir = await tempDir("moleman-run-");
  const workflowPath = path.join(dir, "moleman.yaml");

  await fs.writeFile(
    path.join(dir, "agents.yaml"),
    `agents:
  echo:
    type: generic
    command: "printf"
    args: []
    capture: [stdout, stderr, exitCode]
`,
    "utf8",
  );

  await fs.writeFile(
    workflowPath,
    `version: 1
workflow:
  - type: agent
    name: first
    agent: echo
    input:
      prompt: "hello"
    output:
      toNext: true
`,
    "utf8",
  );

  const workflowConfig = await loadWorkflow(workflowPath);
  const result = await run(workflowConfig, workflowPath, {});

  assert.ok(result.runDir.length > 0);
  await assert.doesNotReject(() => fs.stat(path.join(result.runDir, "summary.md")));
  await assert.doesNotReject(() => fs.stat(path.join(result.runDir, "nodes", "first", "meta.json")));
});

test("run loop exhaustion returns error", async () => {
  const dir = await tempDir("moleman-run-");
  const workflowPath = path.join(dir, "moleman.yaml");

  await fs.writeFile(
    path.join(dir, "agents.yaml"),
    `agents:
  fail:
    type: generic
    command: "false"
    capture: [stdout, stderr, exitCode]
`,
    "utf8",
  );

  await fs.writeFile(
    workflowPath,
    `version: 1
workflow:
  - type: loop
    maxIters: 2
    until: 'outputs.__previous__ == "ok"'
    body:
      - type: agent
        name: fail_once
        agent: fail
        input:
          prompt: "ignored"
        output:
          toNext: true
`,
    "utf8",
  );

  const workflowConfig = await loadWorkflow(workflowPath);
  await assert.rejects(() => run(workflowConfig, workflowPath, {}));
});
