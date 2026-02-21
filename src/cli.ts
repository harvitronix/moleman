#!/usr/bin/env node

import path from "node:path";
import { Command } from "commander";
import { agentNames, loadWorkflow, printWorkflow } from "./workflow.js";
import { doctor } from "./doctor.js";
import { error, info, setLogLevel } from "./logger.js";
import { fileExists, homeWorkflowPath, run } from "./run.js";
import { VERSION } from "./version.js";

async function main(): Promise<void> {
  setLogLevel("info");

  const program = new Command();
  program
    .name("moleman")
    .description("agent-assisted workflow runner")
    .version(VERSION, "-v, --version", "print the version")
    .showHelpAfterError();

  const runCommand = program
    .command("run")
    .description("Execute the workflow")
    .option("--prompt <prompt>", "prompt text")
    .option("--prompt-file <path>", "prompt file path")
    .option("--workdir <dir>", "working directory")
    .option("--workflow <path>", "workflow file path")
    .option("--dry-run", "resolve and plan without executing", false)
    .option("--verbose", "verbose logging", false)
    .action(async (opts: {
      prompt?: string;
      promptFile?: string;
      workdir?: string;
      workflow?: string;
      dryRun?: boolean;
      verbose?: boolean;
    }) => {
      if (opts.verbose) {
        setLogLevel("debug");
      }

      const workflowPath = await resolveWorkflowPath(opts.workflow ?? "", opts.workdir ?? "");
      const workflowConfig = await loadWorkflow(workflowPath);
      const result = await run(workflowConfig, workflowPath, {
        prompt: opts.prompt,
        promptFile: opts.promptFile,
        workdir: opts.workdir,
        dryRun: opts.dryRun,
        verbose: opts.verbose,
      });

      info("run succeeded", { path: result.runDir });
    });

  runCommand.addHelpText(
    "after",
    `
Examples:
  moleman run --prompt "Fix the lint errors"
  moleman run --workflow ./moleman.yaml --prompt-file ./prompt.md`,
  );

  program
    .command("agents")
    .alias("pipelines")
    .description("List agent profiles in the workflow")
    .option("--workflow <path>", "workflow file path")
    .option("--workdir <dir>", "working directory")
    .action(async (opts: { workflow?: string; workdir?: string }) => {
      const workflowPath = await resolveWorkflowPath(opts.workflow ?? "", opts.workdir ?? "");
      const workflowConfig = await loadWorkflow(workflowPath);
      for (const name of agentNames(workflowConfig)) {
        process.stdout.write(`${name}\n`);
      }
    });

  program
    .command("explain")
    .description("Print the resolved workflow")
    .option("--workflow <path>", "workflow file path")
    .option("--workdir <dir>", "working directory")
    .action(async (opts: { workflow?: string; workdir?: string }) => {
      const workflowPath = await resolveWorkflowPath(opts.workflow ?? "", opts.workdir ?? "");
      const workflowConfig = await loadWorkflow(workflowPath);
      printWorkflow(workflowConfig.workflow);
    });

  program
    .command("doctor")
    .description("Validate environment and workflow")
    .option("--workdir <dir>", "working directory")
    .option("--workflow <path>", "workflow file path")
    .action(async (opts: { workdir?: string; workflow?: string }) => {
      const workflowPath = await resolveWorkflowPath(opts.workflow ?? "", opts.workdir ?? "");
      await doctor(workflowPath);
      info("doctor ok");
    });

  program
    .command("version")
    .description("Print version")
    .action(() => {
      process.stdout.write(`${VERSION}\n`);
    });

  if (process.argv.length <= 2) {
    program.outputHelp();
    return;
  }

  await program.parseAsync(process.argv);
}

async function resolveWorkflowPath(workflowPath: string, workdir: string): Promise<string> {
  if (workflowPath) {
    if (!workdir) {
      return workflowPath;
    }
    if (path.isAbsolute(workflowPath)) {
      return workflowPath;
    }
    return path.join(workdir, workflowPath);
  }

  const baseDir = workdir || ".";

  const primary = path.join(baseDir, "moleman.yaml");
  if (await fileExists(primary)) {
    return primary;
  }

  const fallback = path.join(baseDir, ".moleman", "workflows", "default.yaml");
  if (await fileExists(fallback)) {
    return fallback;
  }

  const homeFallback = homeWorkflowPath();
  if (homeFallback && (await fileExists(homeFallback))) {
    return homeFallback;
  }

  return primary;
}

main().catch((err: unknown) => {
  if (err instanceof Error) {
    error(err.message);
  } else {
    error(String(err));
  }
  process.exitCode = 1;
});
