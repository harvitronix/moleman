import { constants as fsConstants, promises as fs } from "node:fs";
import os from "node:os";
import path from "node:path";
import { executeWorkflow } from "./execute.js";
import { workflowDir } from "./workflow.js";
import { info } from "./logger.js";
import { renderTemplate } from "./template.js";
import type { AgentConfig, RunContext, RunOptions, RunResult, WorkflowConfig, WorkflowItem } from "./types.js";

export async function run(workflowConfig: WorkflowConfig, workflowPath: string, opts: RunOptions): Promise<RunResult> {
  let workdir = opts.workdir;
  if (!workdir) {
    workdir = workflowDir(workflowPath);
    if (!workdir) {
      workdir = ".";
    }
  }

  const input = await loadPrompt(opts.prompt ?? "", opts.promptFile ?? "");
  const runId = `${formatTimestamp(new Date())}-workflow`;
  const runDir = path.join(workdir, ".moleman", "runs", runId);

  await fs.mkdir(runDir, { recursive: true }).catch((err: unknown) => {
    if (err instanceof Error) {
      throw new Error(`create run dir: ${err.message}`);
    }
    throw err;
  });

  await writeArtifactsSkeleton(runDir, input, workflowConfig.workflow);

  const ctx: RunContext = {
    input,
    outputs: {},
    sessions: {},
    runDir,
    workdir,
    verbose: Boolean(opts.verbose),
    nodeResults: [],
    lastOutput: "",
  };

  try {
    await ensureAgentCommands(workflowConfig, ctx.workdir);
  } catch (err) {
    await writeSummary(runDir, "failed", err, ctx);
    throw err;
  }

  info("run started", { nodes: workflowConfig.workflow.length });
  info("run artifacts", { path: runDir });

  if (opts.dryRun) {
    await writeSummary(runDir, "dry-run", undefined, ctx);
    return { runDir };
  }

  try {
    await executeWorkflow(ctx, workflowConfig, workflowConfig.workflow);
  } catch (err) {
    await writeSummary(runDir, "failed", err, ctx);
    throw err;
  }

  await writeSummary(runDir, "success", undefined, ctx);
  return { runDir };
}

export async function ensureAgentCommands(workflowConfig: WorkflowConfig, workdir: string): Promise<void> {
  const usedAgents = new Set<string>();
  collectAgentNames(workflowConfig.workflow, usedAgents);

  for (const name of usedAgents) {
    const agent = workflowConfig.agents[name];
    if (!agent) {
      continue;
    }

    const command = resolveAgentCommand(agent);
    if (!command) {
      throw new Error(`agent ${name} has no command configured`);
    }

    await commandAvailable(command, workdir).catch((err: unknown) => {
      if (err instanceof Error) {
        throw new Error(`agent ${name} command not found: ${command} (${err.message})`);
      }
      throw err;
    });

    await validateOutputSchema(agent.outputSchema ?? "", workdir).catch((err: unknown) => {
      if (err instanceof Error) {
        throw new Error(`agent ${name} output schema error: ${err.message}`);
      }
      throw err;
    });
  }
}

function collectAgentNames(items: WorkflowItem[], used: Set<string>): void {
  for (const item of items) {
    if (item.type === "agent") {
      if (item.agent) {
        used.add(item.agent);
      }
      continue;
    }

    if (item.type === "loop") {
      collectAgentNames(item.body, used);
    }
  }
}

function resolveAgentCommand(agent: AgentConfig): string {
  if (agent.command) {
    return agent.command;
  }

  if (agent.type === "codex") {
    return "codex";
  }
  if (agent.type === "claude") {
    return "claude";
  }
  return "";
}

async function commandAvailable(command: string, workdir: string): Promise<void> {
  if (path.isAbsolute(command)) {
    await fs.access(command, fsConstants.F_OK);
    return;
  }

  if (command.includes(path.sep) || command.includes("/")) {
    const cmdPath = path.isAbsolute(command) ? command : path.join(workdir, command);
    await fs.access(cmdPath, fsConstants.F_OK);
    return;
  }

  const resolved = await lookPath(command);
  if (!resolved) {
    throw new Error("not in PATH");
  }
}

async function validateOutputSchema(schemaPath: string, workdir: string): Promise<void> {
  if (!schemaPath) {
    return;
  }

  let resolved = renderTemplate(schemaPath, {});
  if (!path.isAbsolute(resolved) && workdir) {
    resolved = path.join(workdir, resolved);
  }

  await fs.access(resolved, fsConstants.F_OK);
}

async function writeArtifactsSkeleton(runDir: string, input: string, workflow: WorkflowItem[]): Promise<void> {
  await fs.writeFile(path.join(runDir, "input.md"), input, "utf8").catch((err: unknown) => {
    if (err instanceof Error) {
      throw new Error(`write input.md: ${err.message}`);
    }
    throw err;
  });

  await fs
    .writeFile(path.join(runDir, "resolved-workflow.json"), `${JSON.stringify(workflow, null, 2)}\n`, "utf8")
    .catch((err: unknown) => {
      if (err instanceof Error) {
        throw new Error(`write resolved workflow: ${err.message}`);
      }
      throw err;
    });

  for (const dir of [path.join(runDir, "nodes"), path.join(runDir, "diffs")]) {
    await fs.mkdir(dir, { recursive: true }).catch((err: unknown) => {
      if (err instanceof Error) {
        throw new Error(`create dir ${dir}: ${err.message}`);
      }
      throw err;
    });
  }
}

async function writeSummary(runDir: string, status: string, err: unknown, ctx: RunContext): Promise<void> {
  const payload = {
    status,
    error: err instanceof Error ? err.message : undefined,
    time: new Date().toISOString(),
    nodes: ctx.nodeResults,
  };

  const content = `# moleman Run Summary\n\n${JSON.stringify(payload, null, 2)}\n`;
  await fs.writeFile(path.join(runDir, "summary.md"), content, "utf8");
}

export async function loadPrompt(prompt: string, promptFile: string): Promise<string> {
  if (prompt && promptFile) {
    throw new Error("provide only one of --prompt or --prompt-file");
  }

  if (promptFile) {
    return fs.readFile(promptFile, "utf8").catch((err: unknown) => {
      if (err instanceof Error) {
        throw new Error(`read prompt file: ${err.message}`);
      }
      throw err;
    });
  }

  return prompt;
}

async function lookPath(command: string): Promise<string | null> {
  const pathEnv = process.env.PATH ?? "";
  const parts = pathEnv.split(path.delimiter).filter(Boolean);

  if (parts.length === 0) {
    return null;
  }

  const extensions = process.platform === "win32" ? (process.env.PATHEXT ?? ".EXE;.CMD;.BAT").split(";") : [""];

  for (const dir of parts) {
    for (const ext of extensions) {
      const candidate = path.join(dir, `${command}${ext}`);
      try {
        await fs.access(candidate, fsConstants.X_OK);
        return candidate;
      } catch {
        // continue
      }
    }
  }

  return null;
}

function formatTimestamp(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  const hour = String(date.getHours()).padStart(2, "0");
  const minute = String(date.getMinutes()).padStart(2, "0");
  const second = String(date.getSeconds()).padStart(2, "0");
  return `${year}${month}${day}-${hour}${minute}${second}`;
}

export function homeWorkflowPath(): string {
  return path.join(os.homedir(), ".moleman", "workflows", "default.yaml");
}

export async function fileExists(filePath: string): Promise<boolean> {
  try {
    const stats = await fs.stat(filePath);
    return stats.isFile();
  } catch {
    return false;
  }
}
