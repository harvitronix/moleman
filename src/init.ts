import { promises as fs } from "node:fs";
import path from "node:path";

const defaultAgents = `agents:
  codex:
    type: codex
    args: ["--full-auto"]
    timeout: 45m
    capture: [stdout, stderr, exitCode]
`;

const defaultWorkflow = `version: 1

workflow:
  - type: agent
    name: write
    agent: codex
    input:
      from: input
    output:
      stdout: true
`;

export async function init(workflowPath: string, force: boolean): Promise<void> {
  if (!workflowPath) {
    throw new Error("workflow path is empty");
  }

  if (!force) {
    const exists = await fs
      .stat(workflowPath)
      .then(() => true)
      .catch(() => false);
    if (exists) {
      throw new Error(`workflow already exists: ${workflowPath}`);
    }
  }

  const workflowDir = path.dirname(workflowPath);
  await fs.mkdir(workflowDir, { recursive: true }).catch((err: unknown) => {
    if (err instanceof Error) {
      throw new Error(`create workflow dir: ${err.message}`);
    }
    throw err;
  });

  const agentsPath = path.join(workflowDir, "agents.yaml");
  const agentsExists = await fs
    .stat(agentsPath)
    .then(() => true)
    .catch(() => false);

  if (!agentsExists || force) {
    await fs.writeFile(agentsPath, defaultAgents, "utf8").catch((err: unknown) => {
      if (err instanceof Error) {
        throw new Error(`write agents.yaml: ${err.message}`);
      }
      throw err;
    });
  }

  await fs.writeFile(workflowPath, defaultWorkflow, "utf8").catch((err: unknown) => {
    if (err instanceof Error) {
      throw new Error(`write workflow: ${err.message}`);
    }
    throw err;
  });
}
