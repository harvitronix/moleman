import { promises as fs } from "node:fs";
import { loadConfig, validateConfig, configDir } from "./config.js";
import { ensureAgentCommands } from "./run.js";

export async function doctor(configPath: string): Promise<void> {
  await fs.stat(configPath).catch(() => {
    throw new Error(`config not found: ${configPath}`);
  });

  const cfg = await loadConfig(configPath);
  validateConfig(cfg);

  let workdir = configDir(configPath);
  if (!workdir) {
    workdir = ".";
  }

  await ensureAgentCommands(cfg, workdir);
}
