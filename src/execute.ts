import { createWriteStream, promises as fs } from "node:fs";
import path from "node:path";
import { spawn } from "node:child_process";
import { finished } from "node:stream/promises";
import { performance } from "node:perf_hooks";
import { evalCondition } from "./expr.js";
import { debug, info, warn } from "./logger.js";
import { renderTemplate } from "./template.js";
import type {
  AgentConfig,
  Config,
  NodeResult,
  RunContext,
  SessionSpec,
  WorkflowItem,
} from "./types.js";
import { templateData } from "./types.js";

export async function executeWorkflow(ctx: RunContext, cfg: Config, items: WorkflowItem[]): Promise<void> {
  for (const item of items) {
    if (item.type === "agent") {
      await executeAgentNode(ctx, cfg, item);
      continue;
    }
    if (item.type === "loop") {
      await executeLoop(ctx, cfg, item);
      continue;
    }
    throw new Error(`unknown workflow type: ${(item as { type?: string }).type ?? ""}`);
  }
}

async function executeLoop(ctx: RunContext, cfg: Config, item: Extract<WorkflowItem, { type: "loop" }>): Promise<void> {
  for (let i = 0; i < item.maxIters; i += 1) {
    if (ctx.verbose) {
      debug(`loop iteration ${i + 1}/${item.maxIters}`);
    }

    await executeWorkflow(ctx, cfg, item.body);

    const cond = evalCondition(item.until, templateData(ctx));
    if (cond) {
      info("loop condition met", { iteration: i + 1, max: item.maxIters });
      return;
    }
  }

  throw new Error("loop exhausted without meeting condition");
}

async function executeAgentNode(
  ctx: RunContext,
  cfg: Config,
  item: Extract<WorkflowItem, { type: "agent" }>,
): Promise<void> {
  const agent = cfg.agents[item.agent];
  if (!agent) {
    throw new Error(`unknown agent: ${item.agent}`);
  }

  const input = await resolveInput(ctx, item.input);
  const { command, args } = buildAgentCommand(ctx, agent, item, input);

  const stepDir = await nodeRunDir(ctx.runDir, item.name);
  const { stdoutBuf, stderrBuf, exitCode, duration } = await runCommand(
    ctx,
    item.name,
    item.agent,
    command,
    args,
    agent,
    stepDir,
    input,
  );

  await handleOutput(ctx, item, stdoutBuf);

  if (agent.type === "claude") {
    updateClaudeSession(ctx, stdoutBuf);
  }

  const nodeResult: NodeResult = {
    name: item.name,
    agent: item.agent,
    exitCode,
    duration,
    command: [command, ...args].join(" "),
  };
  ctx.nodeResults.push(nodeResult);

  if (exitCode !== 0) {
    const stderrSummary = summarizeStderr(stderrBuf);
    const stderrPath = path.join(stepDir, "stderr.log");
    if (stderrSummary) {
      throw new Error(`node failed: ${item.name} (exit ${exitCode}). stderr: ${stderrSummary} (see ${stderrPath})`);
    }
    throw new Error(`node failed: ${item.name} (exit ${exitCode}). see ${stderrPath}`);
  }

  info("node done", {
    name: item.name,
    agent: item.agent,
    exit: exitCode,
    duration,
  });
}

async function resolveInput(ctx: RunContext, input: { prompt?: string; file?: string; from?: string }): Promise<string> {
  const data = templateData(ctx);

  if (input.prompt) {
    return renderTemplate(input.prompt, data);
  }

  if (input.file) {
    const filePath = renderTemplate(input.file, data);
    return fs.readFile(filePath, "utf8").catch((err: unknown) => {
      if (err instanceof Error) {
        throw new Error(`read input file: ${err.message}`);
      }
      throw err;
    });
  }

  if (input.from) {
    switch (input.from) {
      case "previous":
      case "prev":
      case "last":
        return outputAsString(ctx.outputs.__previous__);
      case "input":
        return ctx.input;
      default: {
        if (!(input.from in ctx.outputs)) {
          throw new Error(`input from unknown node: ${input.from}`);
        }
        return outputAsString(ctx.outputs[input.from]);
      }
    }
  }

  throw new Error("input is empty");
}

function buildAgentCommand(
  ctx: RunContext,
  agent: AgentConfig,
  item: Extract<WorkflowItem, { type: "agent" }>,
  input: string,
): { command: string; args: string[] } {
  let command = agent.command;
  if (!command) {
    if (agent.type === "codex") {
      command = "codex";
    } else if (agent.type === "claude") {
      command = "claude";
    } else {
      throw new Error(`unsupported agent type: ${agent.type}`);
    }
  }

  const session = effectiveSession(agent.session, item.session ?? {});
  const args: string[] = [];
  const data = templateData(ctx);

  let outputSchema = agent.outputSchema;
  if (outputSchema) {
    outputSchema = renderTemplate(outputSchema, data);
  }

  let outputFile = agent.outputFile;
  if (outputFile) {
    outputFile = renderTemplate(outputFile, data);
  }

  switch (agent.type) {
    case "codex": {
      let resumeLast = session.resume === "last";
      if (resumeLast && (outputSchema || outputFile)) {
        warn("codex resume disabled for output schema/file", {
          node: item.name,
          agent: item.agent,
        });
        resumeLast = false;
      }

      if (resumeLast) {
        args.push("exec", "resume", "--last");
      } else {
        args.push("exec");
      }
      args.push(...buildModelArgs(agent));
      args.push(...(agent.args ?? []));
      if (outputSchema) {
        args.push("--output-schema", outputSchema);
      }
      if (outputFile) {
        args.push("--output-last-message", outputFile);
      }
      args.push(input);
      break;
    }
    case "claude": {
      args.push("-p", input);
      args.push(...buildModelArgs(agent));
      args.push(...(agent.args ?? []));
      if (session.resume === "last") {
        const sessionId = ctx.sessions.claude;
        if (!sessionId) {
          throw new Error("claude resume requested but no session_id is available");
        }
        args.push("--resume", sessionId);
      }
      break;
    }
    case "generic": {
      if (!command) {
        throw new Error("generic agent requires command");
      }
      args.push(...(agent.args ?? []));
      if (input) {
        args.push(input);
      }
      break;
    }
    default:
      throw new Error(`unsupported agent type: ${(agent as AgentConfig).type}`);
  }

  return { command, args };
}

function buildModelArgs(agent: AgentConfig): string[] {
  const args: string[] = [];
  if (agent.type === "codex") {
    if (agent.model) {
      args.push("--model", agent.model);
    }
    if (agent.thinking) {
      args.push("-c", `model_reasoning_effort=${agent.thinking}`);
    }
  }
  if (agent.type === "claude" && agent.model) {
    args.push("--model", agent.model);
  }
  return args;
}

function effectiveSession(agentSession: SessionSpec | undefined, nodeSession: SessionSpec): SessionSpec {
  if (nodeSession.resume) {
    return nodeSession;
  }
  if (agentSession) {
    return agentSession;
  }
  return { resume: "new" };
}

async function runCommand(
  ctx: RunContext,
  nodeName: string,
  agentName: string,
  command: string,
  args: string[],
  agent: AgentConfig,
  stepDir: string,
  input: string,
): Promise<{ stdoutBuf: Buffer; stderrBuf: Buffer; exitCode: number; duration: string }> {
  info("node start", {
    command,
    args: args.join(" "),
  });

  const timeoutMs = parseDuration(agent.timeout ?? "");
  const stdoutPath = path.join(stepDir, "stdout.log");
  const stderrPath = path.join(stepDir, "stderr.log");

  const stdoutFile = createWriteStream(stdoutPath);
  const stderrFile = createWriteStream(stderrPath);

  const captureStdout = shouldCapture(agent.capture, "stdout");
  const captureStderr = shouldCapture(agent.capture, "stderr");
  let printStdout = shouldPrint(agent.print, "stdout");
  let printStderr = shouldPrint(agent.print, "stderr");

  if (ctx.verbose) {
    printStdout = true;
    printStderr = true;
  }

  const stdoutChunks: Buffer[] = [];
  const stderrChunks: Buffer[] = [];

  const stdoutTracker = { wrote: false, lastByte: 0 };
  const stderrTracker = { wrote: false, lastByte: 0 };

  const env = {
    ...process.env,
    ...(agent.env ?? {}),
  };

  let timedOut = false;

  const start = performance.now();
  const child = spawn(command, args, {
    cwd: ctx.workdir,
    env,
    stdio: ["pipe", "pipe", "pipe"],
  });

  const stdoutStream = child.stdout;
  const stderrStream = child.stderr;
  const stdinStream = child.stdin;
  if (!stdoutStream || !stderrStream || !stdinStream) {
    throw new Error("failed to initialize process streams");
  }

  let timer: NodeJS.Timeout | undefined;
  if (timeoutMs > 0) {
    timer = setTimeout(() => {
      timedOut = true;
      child.kill("SIGTERM");
    }, timeoutMs);
  }

  stdoutStream.on("data", (chunk: Buffer | string) => {
    const data = typeof chunk === "string" ? Buffer.from(chunk) : chunk;
    stdoutFile.write(data);
    if (captureStdout) {
      stdoutChunks.push(data);
    }
    if (printStdout) {
      if (!stdoutTracker.wrote) {
        process.stdout.write("\n");
      }
      process.stdout.write(data);
    }
    if (data.length > 0) {
      stdoutTracker.wrote = true;
      stdoutTracker.lastByte = data[data.length - 1];
    }
  });

  stderrStream.on("data", (chunk: Buffer | string) => {
    const data = typeof chunk === "string" ? Buffer.from(chunk) : chunk;
    stderrFile.write(data);
    if (captureStderr) {
      stderrChunks.push(data);
    }
    if (printStderr) {
      if (!stderrTracker.wrote) {
        process.stderr.write("\n");
      }
      process.stderr.write(data);
    }
    if (data.length > 0) {
      stderrTracker.wrote = true;
      stderrTracker.lastByte = data[data.length - 1];
    }
  });

  if (input && agent.type === "claude" && !args.includes("-p")) {
    stdinStream.write(input);
  }
  stdinStream.end();

  const exitCode = await new Promise<number>((resolve, reject) => {
    child.once("error", (err: NodeJS.ErrnoException) => {
      if (err.code === "ENOENT") {
        resolve(127);
        return;
      }
      reject(err);
    });

    child.once("close", (code) => {
      resolve(code ?? (timedOut ? 124 : 1));
    });
  });

  if (timer) {
    clearTimeout(timer);
  }

  stdoutFile.end();
  stderrFile.end();

  await Promise.all([finished(stdoutFile), finished(stderrFile)]);

  if (printStdout && stdoutTracker.wrote && stdoutTracker.lastByte !== 10) {
    process.stdout.write("\n");
  }
  if (printStderr && stderrTracker.wrote && stderrTracker.lastByte !== 10) {
    process.stderr.write("\n");
  }

  const duration = formatDuration(performance.now() - start);

  const meta: NodeResult = {
    name: nodeName,
    agent: agentName,
    exitCode: timedOut ? 124 : exitCode,
    duration,
    command: [command, ...args].join(" "),
  };
  await writeMeta(stepDir, meta, stdoutPath, stderrPath);

  return {
    stdoutBuf: Buffer.concat(stdoutChunks),
    stderrBuf: Buffer.concat(stderrChunks),
    exitCode: timedOut ? 124 : exitCode,
    duration,
  };
}

async function handleOutput(
  ctx: RunContext,
  item: Extract<WorkflowItem, { type: "agent" }>,
  stdout: Buffer,
): Promise<void> {
  const output = stdout.toString("utf8");

  if (item.output.toNext) {
    ctx.lastOutput = output;
    ctx.outputs.__previous__ = output;
    if (item.name) {
      ctx.outputs[item.name] = output;
    }

    const parsed = parseJSONOutput(stdout);
    if (parsed !== null) {
      const normalized = normalizeStructuredOutput(parsed);
      ctx.outputs.__previous_json__ = normalized;
      if (item.name) {
        ctx.outputs[`${item.name}_json`] = normalized;
      }
    }
  }

  if (item.output.file) {
    const renderedPath = renderTemplate(item.output.file, templateData(ctx));
    await fs.writeFile(renderedPath, stdout);
  }

  if (item.output.stdout) {
    process.stdout.write("\n");
    process.stdout.write(output);
    if (stdout.length > 0 && stdout[stdout.length - 1] !== 10) {
      process.stdout.write("\n");
    }
  }
}

function updateClaudeSession(ctx: RunContext, stdout: Buffer): void {
  try {
    const parsed = JSON.parse(stdout.toString("utf8")) as { session_id?: unknown };
    if (typeof parsed.session_id === "string" && parsed.session_id.length > 0) {
      ctx.sessions.claude = parsed.session_id;
    }
  } catch {
    // Ignore non-JSON output.
  }
}

function parseJSONOutput(stdout: Buffer): unknown | null {
  try {
    return JSON.parse(stdout.toString("utf8"));
  } catch {
    return null;
  }
}

function normalizeStructuredOutput(value: unknown): unknown {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return value;
  }

  const obj = value as Record<string, unknown>;
  if ("structured_output" in obj) {
    return obj;
  }

  return {
    structured_output: obj,
    ...obj,
  };
}

function outputAsString(value: unknown): string {
  if (value === null || value === undefined) {
    return "";
  }
  if (typeof value === "string") {
    return value;
  }
  if (Buffer.isBuffer(value)) {
    return value.toString("utf8");
  }
  return JSON.stringify(value);
}

function summarizeStderr(buf: Buffer): string {
  if (!buf || buf.length === 0) {
    return "";
  }

  const text = buf.toString("utf8").trim();
  if (!text) {
    return "";
  }

  const maxLen = 4000;
  if (text.length <= maxLen) {
    return text;
  }
  return `...(truncated)...\n${text.slice(-maxLen)}`;
}

async function nodeRunDir(runDir: string, name: string): Promise<string> {
  const nodeName = name || "node";
  const dir = path.join(runDir, "nodes", nodeName);
  await fs.mkdir(dir, { recursive: true });
  return dir;
}

async function writeMeta(stepDir: string, meta: NodeResult, stdoutPath: string, stderrPath: string): Promise<void> {
  const payload = {
    ...meta,
    stdoutLog: stdoutPath,
    stderrLog: stderrPath,
  };

  await fs.writeFile(path.join(stepDir, "meta.json"), `${JSON.stringify(payload, null, 2)}\n`, "utf8");
}

function shouldCapture(capture: string[] | undefined, value: string): boolean {
  if (!capture || capture.length === 0) {
    return true;
  }
  return capture.includes(value);
}

function shouldPrint(print: string[] | undefined, value: string): boolean {
  if (!print || print.length === 0) {
    return false;
  }
  return print.includes(value);
}

function parseDuration(value: string): number {
  if (!value) {
    return 0;
  }

  const pattern = /([+-]?\d+(?:\.\d+)?)(ns|us|µs|ms|s|m|h)/g;
  let totalMs = 0;
  let lastIndex = 0;
  let matched = false;

  for (const match of value.matchAll(pattern)) {
    if (match.index !== lastIndex) {
      throw new Error(`timeout: invalid duration: ${value}`);
    }
    matched = true;
    lastIndex += match[0].length;

    const amount = Number(match[1]);
    const unit = match[2];
    const multiplier =
      unit === "h"
        ? 3_600_000
        : unit === "m"
          ? 60_000
          : unit === "s"
            ? 1_000
            : unit === "ms"
              ? 1
              : unit === "us" || unit === "µs"
                ? 0.001
                : 0.000001;

    totalMs += amount * multiplier;
  }

  if (!matched || lastIndex !== value.length) {
    throw new Error(`timeout: invalid duration: ${value}`);
  }

  return Math.max(0, Math.round(totalMs));
}

function formatDuration(ms: number): string {
  if (ms < 1_000) {
    return `${Math.round(ms)}ms`;
  }

  const seconds = ms / 1_000;
  return `${seconds.toFixed(3).replace(/\.?0+$/, "")}s`;
}
