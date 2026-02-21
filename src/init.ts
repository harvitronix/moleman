import { promises as fs } from "node:fs";
import path from "node:path";

const defaultAgents = `agents:
  codex:
    type: codex
    args: ["--full-auto"]
    timeout: 45m
    capture: [stdout, stderr, exitCode]
`;

const defaultConfig = `version: 1

workflow:
  - type: agent
    name: write
    agent: codex
    input:
      from: input
    output:
      stdout: true
`;

export async function init(configPath: string, force: boolean): Promise<void> {
  if (!configPath) {
    throw new Error("config path is empty");
  }

  if (!force) {
    const exists = await fs
      .stat(configPath)
      .then(() => true)
      .catch(() => false);
    if (exists) {
      throw new Error(`config already exists: ${configPath}`);
    }
  }

  const configDir = path.dirname(configPath);
  await fs.mkdir(configDir, { recursive: true }).catch((err: unknown) => {
    if (err instanceof Error) {
      throw new Error(`create config dir: ${err.message}`);
    }
    throw err;
  });

  const agentsPath = path.join(configDir, "agents.yaml");
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

  await fs.writeFile(configPath, defaultConfig, "utf8").catch((err: unknown) => {
    if (err instanceof Error) {
      throw new Error(`write config: ${err.message}`);
    }
    throw err;
  });
}
