#!/usr/bin/env node

import path from "node:path";
import { Command } from "commander";
import { agentNames, loadConfig, printWorkflow } from "./config.js";
import { doctor } from "./doctor.js";
import { init } from "./init.js";
import { error, info, setLogLevel } from "./logger.js";
import { fileExists, homeConfigPath, run } from "./run.js";
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
    .option("--config <path>", "config file path")
    .option("--dry-run", "resolve and plan without executing", false)
    .option("--verbose", "verbose logging", false)
    .action(async (opts: {
      prompt?: string;
      promptFile?: string;
      workdir?: string;
      config?: string;
      dryRun?: boolean;
      verbose?: boolean;
    }) => {
      if (opts.verbose) {
        setLogLevel("debug");
      }

      const cfgPath = await resolveConfigPath(opts.config ?? "", opts.workdir ?? "");
      const cfg = await loadConfig(cfgPath);
      const result = await run(cfg, cfgPath, {
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
  moleman run --config ./moleman.yaml --prompt-file ./prompt.md`,
  );

  program
    .command("agents")
    .alias("pipelines")
    .description("List agents in the config")
    .option("--config <path>", "config file path")
    .option("--workdir <dir>", "working directory")
    .action(async (opts: { config?: string; workdir?: string }) => {
      const cfgPath = await resolveConfigPath(opts.config ?? "", opts.workdir ?? "");
      const cfg = await loadConfig(cfgPath);
      for (const name of agentNames(cfg)) {
        process.stdout.write(`${name}\n`);
      }
    });

  program
    .command("explain")
    .description("Print the resolved workflow")
    .option("--config <path>", "config file path")
    .option("--workdir <dir>", "working directory")
    .action(async (opts: { config?: string; workdir?: string }) => {
      const cfgPath = await resolveConfigPath(opts.config ?? "", opts.workdir ?? "");
      const cfg = await loadConfig(cfgPath);
      printWorkflow(cfg.workflow);
    });

  program
    .command("init")
    .description("Create an example config")
    .option("--workdir <dir>", "working directory")
    .option("--force", "overwrite existing config", false)
    .option("--config <path>", "config file path")
    .action(async (opts: { workdir?: string; force?: boolean; config?: string }) => {
      const cfgPath = await resolveConfigPath(opts.config ?? "", opts.workdir ?? "");
      await init(cfgPath, Boolean(opts.force));
      info("created", { path: cfgPath });
    });

  program
    .command("doctor")
    .description("Validate environment and config")
    .option("--workdir <dir>", "working directory")
    .option("--config <path>", "config file path")
    .action(async (opts: { workdir?: string; config?: string }) => {
      const cfgPath = await resolveConfigPath(opts.config ?? "", opts.workdir ?? "");
      await doctor(cfgPath);
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

async function resolveConfigPath(configPath: string, workdir: string): Promise<string> {
  if (configPath) {
    if (!workdir) {
      return configPath;
    }
    if (path.isAbsolute(configPath)) {
      return configPath;
    }
    return path.join(workdir, configPath);
  }

  const baseDir = workdir || ".";

  const primary = path.join(baseDir, "moleman.yaml");
  if (await fileExists(primary)) {
    return primary;
  }

  const fallback = path.join(baseDir, ".moleman", "configs", "default.yaml");
  if (await fileExists(fallback)) {
    return fallback;
  }

  const homeFallback = homeConfigPath();
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
