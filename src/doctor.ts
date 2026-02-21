import { promises as fs } from "node:fs";
import { loadWorkflow, validateWorkflowConfig, workflowDir } from "./workflow.js";
import { ensureAgentCommands } from "./run.js";

export async function doctor(workflowPath: string): Promise<void> {
  await fs.stat(workflowPath).catch(() => {
    throw new Error(`workflow not found: ${workflowPath}`);
  });

  const workflowConfig = await loadWorkflow(workflowPath);
  validateWorkflowConfig(workflowConfig);

  let workdir = workflowDir(workflowPath);
  if (!workdir) {
    workdir = ".";
  }

  await ensureAgentCommands(workflowConfig, workdir);
}
