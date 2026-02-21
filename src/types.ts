export interface Config {
  version: number;
  agents: Record<string, AgentConfig>;
  workflow: WorkflowItem[];
}

export interface AgentConfig {
  extends?: string;
  type: "codex" | "claude" | "generic";
  command?: string;
  model?: string;
  thinking?: string;
  args?: string[];
  outputSchema?: string;
  outputFile?: string;
  env?: Record<string, string>;
  timeout?: string;
  capture?: string[];
  print?: string[];
  session?: SessionSpec;
}

export interface SessionSpec {
  resume?: string;
}

export interface InputSpec {
  prompt?: string;
  file?: string;
  from?: string;
}

export interface OutputSpec {
  toNext?: boolean;
  file?: string;
  stdout?: boolean;
}

export interface WorkflowAgentItem {
  type: "agent";
  name: string;
  agent: string;
  input: InputSpec;
  output: OutputSpec;
  session?: SessionSpec;
}

export interface WorkflowLoopItem {
  type: "loop";
  maxIters: number;
  until: string;
  body: WorkflowItem[];
  name?: string;
  agent?: string;
  input?: InputSpec;
  output?: OutputSpec;
  session?: SessionSpec;
}

export type WorkflowItem = WorkflowAgentItem | WorkflowLoopItem;

export interface RunOptions {
  prompt?: string;
  promptFile?: string;
  workdir?: string;
  dryRun?: boolean;
  verbose?: boolean;
}

export interface RunResult {
  runDir: string;
}

export interface NodeResult {
  name: string;
  agent: string;
  exitCode: number;
  duration: string;
  command: string;
}

export interface RunContext {
  input: string;
  outputs: Record<string, unknown>;
  lastOutput: string;
  sessions: Record<string, string>;
  runDir: string;
  workdir: string;
  verbose: boolean;
  nodeResults: NodeResult[];
}

export function templateData(ctx: RunContext): Record<string, unknown> {
  return {
    input: {
      prompt: ctx.input,
    },
    outputs: ctx.outputs,
    last: ctx.lastOutput,
    sessions: ctx.sessions,
  };
}
