import { promises as fs } from "node:fs";
import path from "node:path";
import YAML from "yaml";
import type { AgentConfig, InputSpec, OutputSpec, WorkflowConfig, WorkflowItem } from "./types.js";

export async function loadWorkflow(workflowPath: string): Promise<WorkflowConfig> {
  const raw = await fs.readFile(workflowPath, "utf8").catch((err: unknown) => {
    if (err instanceof Error) {
      throw new Error(`read workflow: ${err.message}`);
    }
    throw err;
  });

  let parsed: Partial<WorkflowConfig>;
  try {
    parsed = (YAML.parse(raw) ?? {}) as Partial<WorkflowConfig>;
  } catch (err) {
    if (err instanceof Error) {
      throw new Error(`parse yaml: ${err.message}`);
    }
    throw err;
  }

  const baseAgents = await loadBaseAgents(workflowPath);
  const version = Number(parsed.version ?? 0);
  if (version !== 1) {
    throw new Error(`unsupported workflow version: ${version}`);
  }

  const overrides = isRecord(parsed.agents) ? (parsed.agents as Record<string, AgentConfig>) : {};
  const mergedAgents = mergeAgents(baseAgents, overrides);

  const workflowConfig: WorkflowConfig = {
    version,
    agents: mergedAgents,
    workflow: Array.isArray(parsed.workflow) ? (parsed.workflow as WorkflowItem[]) : [],
  };

  validateWorkflowConfig(workflowConfig);
  return workflowConfig;
}

export function agentNames(workflowConfig: WorkflowConfig): string[] {
  return Object.keys(workflowConfig.agents).sort();
}

export function workflowDir(workflowPath: string): string {
  const dir = path.dirname(workflowPath);
  return dir === "." ? "" : dir;
}

export function printWorkflow(workflow: WorkflowItem[]): void {
  const payload = JSON.stringify(workflow, null, 2);
  process.stdout.write(`${payload}\n`);
}

export function validateWorkflowConfig(workflowConfig: WorkflowConfig): void {
  if (Object.keys(workflowConfig.agents).length === 0) {
    throw new Error("agent profiles map is empty");
  }
  if (!Array.isArray(workflowConfig.workflow) || workflowConfig.workflow.length === 0) {
    throw new Error("workflow is empty");
  }

  for (const [name, agent] of Object.entries(workflowConfig.agents)) {
    if (!agent.type) {
      throw new Error(`agent profile ${name} missing runtime type`);
    }
    if (!["codex", "claude", "generic"].includes(agent.type)) {
      throw new Error(`agent profile ${name} has unsupported runtime: ${agent.type}`);
    }
    if (agent.type === "generic" && !agent.command) {
      throw new Error(`agent profile ${name} runtime generic requires command`);
    }
    if (agent.model && agent.type === "generic") {
      throw new Error(`agent profile ${name} model is only supported for codex or claude`);
    }
    if (agent.thinking && agent.type !== "codex") {
      throw new Error(`agent profile ${name} thinking is only supported for codex`);
    }
    if (agent.thinking && !isValidCodexThinking(agent.thinking)) {
      throw new Error(`agent profile ${name} thinking must be one of minimal, low, medium, high, xhigh`);
    }
  }

  const seenNames = new Set<string>();
  validateWorkflowItems(workflowConfig, workflowConfig.workflow, seenNames);
}

function isValidCodexThinking(value: string): boolean {
  return ["minimal", "low", "medium", "high", "xhigh"].includes(value);
}

async function loadBaseAgents(workflowPath: string): Promise<Record<string, AgentConfig>> {
  let dir = workflowDir(workflowPath);
  if (!dir) {
    dir = ".";
  }

  const agentsPath = path.join(dir, "agents.yaml");
  await fs.stat(agentsPath).catch((err: unknown) => {
    if ((err as NodeJS.ErrnoException).code === "ENOENT") {
      throw new Error(`agents.yaml not found: ${agentsPath}`);
    }
    if (err instanceof Error) {
      throw new Error(`stat agents.yaml: ${err.message}`);
    }
    throw err;
  });

  const raw = await fs.readFile(agentsPath, "utf8").catch((err: unknown) => {
    if (err instanceof Error) {
      throw new Error(`read agents.yaml: ${err.message}`);
    }
    throw err;
  });

  let payload: { agents?: Record<string, AgentConfig> };
  try {
    payload = (YAML.parse(raw) ?? {}) as { agents?: Record<string, AgentConfig> };
  } catch (err) {
    if (err instanceof Error) {
      throw new Error(`parse agents.yaml: ${err.message}`);
    }
    throw err;
  }

  if (!isRecord(payload.agents)) {
    return {};
  }
  return payload.agents as Record<string, AgentConfig>;
}

function mergeAgents(
  base: Record<string, AgentConfig>,
  overrides: Record<string, AgentConfig>,
): Record<string, AgentConfig> {
  const merged: Record<string, AgentConfig> = { ...base };

  for (const [name, agent] of Object.entries(overrides)) {
    let baseAgent: AgentConfig = {} as AgentConfig;
    if (agent.extends) {
      const extended = base[agent.extends];
      if (!extended) {
        throw new Error(`agent profile ${name} extends unknown agent profile: ${agent.extends}`);
      }
      baseAgent = extended;
    } else if (merged[name]) {
      baseAgent = merged[name];
    }

    merged[name] = mergeAgentConfig(baseAgent, agent);
  }

  return merged;
}

function mergeAgentConfig(base: AgentConfig, override: AgentConfig): AgentConfig {
  const result: AgentConfig = {
    ...base,
  };

  if (override.type) {
    result.type = override.type;
  }
  if (override.command) {
    result.command = override.command;
  }
  if (override.model) {
    result.model = override.model;
  }
  if (override.thinking) {
    result.thinking = override.thinking;
  }
  if (override.args !== undefined) {
    result.args = override.args;
  }
  if (override.outputSchema) {
    result.outputSchema = override.outputSchema;
  }
  if (override.outputFile) {
    result.outputFile = override.outputFile;
  }
  if (override.env !== undefined) {
    result.env = {
      ...(result.env ?? {}),
      ...override.env,
    };
  }
  if (override.timeout) {
    result.timeout = override.timeout;
  }
  if (override.capture !== undefined) {
    result.capture = override.capture;
  }
  if (override.print !== undefined) {
    result.print = override.print;
  }
  if (override.session !== undefined) {
    result.session = override.session;
  }

  delete result.extends;
  return result;
}

function validateWorkflowItems(workflowConfig: WorkflowConfig, items: WorkflowItem[], seenNames: Set<string>): void {
  items.forEach((rawItem, idx) => {
    const item = rawItem as unknown as Record<string, unknown>;
    const type = String(item.type ?? "");

    if (type === "agent") {
      const agentName = String(item.agent ?? "");
      if (!agentName) {
        throw new Error(`workflow[${idx}] agent profile is required`);
      }
      if (!workflowConfig.agents[agentName]) {
        throw new Error(`workflow[${idx}] references unknown agent profile: ${agentName}`);
      }

      const name = String(item.name ?? "");
      if (!name) {
        throw new Error(`workflow[${idx}] name is required`);
      }
      if (seenNames.has(name)) {
        throw new Error(`duplicate workflow name: ${name}`);
      }
      seenNames.add(name);

      validateInput((item.input ?? {}) as InputSpec, idx);
      validateOutput((item.output ?? {}) as OutputSpec, idx);
      return;
    }

    if (type === "loop") {
      const maxIters = Number(item.maxIters ?? 0);
      if (!(maxIters > 0)) {
        throw new Error(`workflow[${idx}] loop maxIters must be > 0`);
      }
      const until = String(item.until ?? "").trim();
      if (!until) {
        throw new Error(`workflow[${idx}] loop until is required`);
      }
      const body = Array.isArray(item.body) ? (item.body as WorkflowItem[]) : [];
      if (body.length === 0) {
        throw new Error(`workflow[${idx}] loop body is empty`);
      }
      validateWorkflowItems(workflowConfig, body, seenNames);
      return;
    }

    throw new Error(`workflow[${idx}] unknown type: ${type}`);
  });
}

function validateInput(input: InputSpec, idx: number): void {
  let count = 0;
  if (input.prompt) {
    count += 1;
  }
  if (input.file) {
    count += 1;
  }
  if (input.from) {
    count += 1;
  }

  if (count === 0) {
    throw new Error(`workflow[${idx}] input requires one of prompt, file, or from`);
  }
  if (count > 1) {
    throw new Error(`workflow[${idx}] input must specify only one of prompt, file, or from`);
  }
}

function validateOutput(output: OutputSpec, idx: number): void {
  let count = 0;
  if (output.toNext) {
    count += 1;
  }
  if (output.file) {
    count += 1;
  }
  if (output.stdout) {
    count += 1;
  }

  if (count === 0) {
    throw new Error(`workflow[${idx}] output requires one of toNext, file, or stdout`);
  }
  if (count > 1) {
    throw new Error(`workflow[${idx}] output must specify only one of toNext, file, or stdout`);
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}
