package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/muesli/termenv"
	"moleman/internal/moleman"
)

func main() {
	configureLogging()
	log.SetLevel(log.InfoLevel)

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
		log.Error("unknown command", "command", os.Args[1])
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

	if *verbose {
		log.SetLevel(log.DebugLevel)
	}

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
			log.Warn("run artifacts", "path", result.RunDir)
		}
		exitErr(err)
	}

	log.Info("run succeeded", "path", result.RunDir)
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
	log.Info("created", "path", cfgPath)
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
	log.Info("doctor ok")
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
	log.Error(err.Error())
	os.Exit(1)
}

func configureLogging() {
	log.SetReportTimestamp(true)
	log.SetTimeFormat("15:04:05")
	log.SetPrefix("moleman")
	log.SetColorProfile(termenv.TrueColor)

	styles := log.DefaultStyles()
	styles.Levels[log.DebugLevel] = styles.Levels[log.DebugLevel].Foreground(lipgloss.Color("69")).Bold(true)
	styles.Levels[log.InfoLevel] = styles.Levels[log.InfoLevel].Foreground(lipgloss.Color("86")).Bold(true)
	styles.Levels[log.WarnLevel] = styles.Levels[log.WarnLevel].Foreground(lipgloss.Color("220")).Bold(true)
	styles.Levels[log.ErrorLevel] = styles.Levels[log.ErrorLevel].Foreground(lipgloss.Color("196")).Bold(true)
	styles.Prefix = styles.Prefix.Foreground(lipgloss.Color("245")).Bold(true)
	styles.Key = styles.Key.Foreground(lipgloss.Color("244"))
	log.SetStyles(styles)
}
