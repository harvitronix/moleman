package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"moleman/internal/moleman"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "run":
		runCmd(os.Args[2:])
	case "pipelines":
		pipelinesCmd(os.Args[2:])
	case "explain":
		explainCmd(os.Args[2:])
	case "init":
		initCmd(os.Args[2:])
	case "doctor":
		doctorCmd(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Print(`moleman - agent-assisted workflow runner

Usage:
  moleman run [flags]
  moleman pipelines [flags]
  moleman explain --pipeline <name> [flags]
  moleman init [flags]
  moleman doctor [flags]
`)
}

func runCmd(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	pipeline := fs.String("pipeline", "default", "pipeline name")
	prompt := fs.String("prompt", "", "prompt text")
	promptFile := fs.String("prompt-file", "", "prompt file path")
	workdir := fs.String("workdir", "", "working directory")
	configPath := fs.String("config", "", "config file path")
	dryRun := fs.Bool("dry-run", false, "resolve and plan without executing")
	verbose := fs.Bool("verbose", false, "verbose logging")
	fs.Parse(args)

	cfgPath := resolveConfigPath(*configPath, *workdir)
	cfg, err := moleman.LoadConfig(cfgPath)
	if err != nil {
		exitErr(err)
	}

	runOpts := moleman.RunOptions{
		Pipeline:   *pipeline,
		Prompt:     *prompt,
		PromptFile: *promptFile,
		Workdir:    *workdir,
		DryRun:     *dryRun,
		Verbose:    *verbose,
	}

	result, err := moleman.Run(cfg, cfgPath, runOpts)
	if err != nil {
		if result != nil && result.RunDir != "" {
			fmt.Fprintf(os.Stderr, "run artifacts: %s\n", result.RunDir)
		}
		exitErr(err)
	}

	fmt.Printf("run succeeded: %s\n", result.RunDir)
}

func pipelinesCmd(args []string) {
	fs := flag.NewFlagSet("pipelines", flag.ExitOnError)
	configPath := fs.String("config", "", "config file path")
	workdir := fs.String("workdir", "", "working directory")
	fs.Parse(args)

	cfgPath := resolveConfigPath(*configPath, *workdir)
	cfg, err := moleman.LoadConfig(cfgPath)
	if err != nil {
		exitErr(err)
	}

	names := moleman.PipelineNames(cfg)
	for _, name := range names {
		fmt.Println(name)
	}
}

func explainCmd(args []string) {
	fs := flag.NewFlagSet("explain", flag.ExitOnError)
	pipeline := fs.String("pipeline", "default", "pipeline name")
	configPath := fs.String("config", "", "config file path")
	workdir := fs.String("workdir", "", "working directory")
	fs.Parse(args)

	cfgPath := resolveConfigPath(*configPath, *workdir)
	cfg, err := moleman.LoadConfig(cfgPath)
	if err != nil {
		exitErr(err)
	}

	plan, err := moleman.ResolvePlan(cfg, *pipeline)
	if err != nil {
		exitErr(err)
	}

	if err := moleman.PrintPlan(plan); err != nil {
		exitErr(err)
	}
}

func initCmd(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	workdir := fs.String("workdir", "", "working directory")
	force := fs.Bool("force", false, "overwrite existing config")
	configPath := fs.String("config", "", "config file path")
	fs.Parse(args)

	cfgPath := resolveConfigPath(*configPath, *workdir)
	if err := moleman.Init(cfgPath, *force); err != nil {
		exitErr(err)
	}
	fmt.Printf("created: %s\n", cfgPath)
}

func doctorCmd(args []string) {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	workdir := fs.String("workdir", "", "working directory")
	configPath := fs.String("config", "", "config file path")
	fs.Parse(args)

	cfgPath := resolveConfigPath(*configPath, *workdir)
	if err := moleman.Doctor(cfgPath); err != nil {
		exitErr(err)
	}
	fmt.Println("doctor ok")
}

func resolveConfigPath(configPath, workdir string) string {
	if configPath != "" {
		if workdir == "" {
			return configPath
		}
		if filepath.IsAbs(configPath) {
			return configPath
		}
		return filepath.Join(workdir, configPath)
	}

	baseDir := workdir
	if baseDir == "" {
		baseDir = "."
	}

	primary := filepath.Join(baseDir, "moleman.yaml")
	if fileExists(primary) {
		return primary
	}

	fallback := filepath.Join(baseDir, ".moleman", "configs", "default.yaml")
	if fileExists(fallback) {
		return fallback
	}

	homeFallback := homeConfigPath()
	if homeFallback != "" && fileExists(homeFallback) {
		return homeFallback
	}

	return primary
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func homeConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".moleman", "configs", "default.yaml")
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, err.Error())
	os.Exit(1)
}
