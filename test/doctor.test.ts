import test from "node:test";
import assert from "node:assert/strict";
import { promises as fs } from "node:fs";
import os from "node:os";
import path from "node:path";
import { doctor } from "../src/doctor.js";

async function tempDir(prefix: string): Promise<string> {
  return fs.mkdtemp(path.join(os.tmpdir(), prefix));
}

test("doctor reports missing workflow", async () => {
  const dir = await tempDir("moleman-doctor-");
  const missingPath = path.join(dir, "missing.yaml");

  await assert.rejects(() => doctor(missingPath), /workflow not found/);
});

test("doctor surfaces validation errors", async () => {
  const dir = await tempDir("moleman-doctor-");
  const workflowPath = path.join(dir, "moleman.yaml");
  const agentsPath = path.join(dir, "agents.yaml");

  await fs.writeFile(agentsPath, "agents: {}\n", "utf8");
  await fs.writeFile(
    workflowPath,
    `version: 1
agents: {}
workflow: []
`,
    "utf8",
  );

  await assert.rejects(() => doctor(workflowPath), /agent profiles map is empty/);
});
