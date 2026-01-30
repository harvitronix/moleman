package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/muesli/termenv"
	"github.com/urfave/cli/v2"
	"moleman/internal/moleman"
)

func main() {
	configureLogging()
	log.SetLevel(log.InfoLevel)

	app := &cli.App{
		Name:                 "moleman",
		Usage:                "agent-assisted workflow runner",
		Version:              Version,
		EnableBashCompletion: true,
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "version", Aliases: []string{"v"}, Usage: "print version"},
		},
		Action: func(c *cli.Context) error {
			if c.Bool("version") {
				fmt.Println(Version)
				return nil
			}
			return cli.ShowAppHelp(c)
		},
		Commands: []*cli.Command{
			runCommand(),
			pipelinesCommand(),
			explainCommand(),
			initCommand(),
			doctorCommand(),
			versionCommand(),
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}
}

func runCommand() *cli.Command {
	return &cli.Command{
		Name:      "run",
		Usage:     "Execute the workflow",
		UsageText: "moleman run [flags]\n\nExamples:\n  moleman run --prompt \"Fix the lint errors\"\n  moleman run --config ./moleman.yaml --prompt-file ./prompt.md",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "prompt", Usage: "prompt text"},
			&cli.StringFlag{Name: "prompt-file", Usage: "prompt file path"},
			&cli.StringFlag{Name: "workdir", Usage: "working directory"},
			&cli.StringFlag{Name: "config", Usage: "config file path"},
			&cli.BoolFlag{Name: "dry-run", Usage: "resolve and plan without executing"},
			&cli.BoolFlag{Name: "verbose", Usage: "verbose logging"},
		},
		Action: func(c *cli.Context) error {
			if c.Bool("verbose") {
				log.SetLevel(log.DebugLevel)
			}

			cfgPath := resolveConfigPath(c.String("config"), c.String("workdir"))
			cfg, err := moleman.LoadConfig(cfgPath)
			if err != nil {
				return err
			}

			runOpts := moleman.RunOptions{
				Prompt:     c.String("prompt"),
				PromptFile: c.String("prompt-file"),
				Workdir:    c.String("workdir"),
				DryRun:     c.Bool("dry-run"),
				Verbose:    c.Bool("verbose"),
			}

			result, err := moleman.Run(cfg, cfgPath, runOpts)
			if err != nil {
				if result != nil && result.RunDir != "" {
					log.Warn("run artifacts", "path", result.RunDir)
				}
				return err
			}

			log.Info("run succeeded", "path", result.RunDir)
			return nil
		},
	}
}

func pipelinesCommand() *cli.Command {
	return &cli.Command{
		Name:      "agents",
		Aliases:   []string{"pipelines"},
		Usage:     "List agents in the config",
		UsageText: "moleman agents [flags]",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "config", Usage: "config file path"},
			&cli.StringFlag{Name: "workdir", Usage: "working directory"},
		},
		Action: func(c *cli.Context) error {
			cfgPath := resolveConfigPath(c.String("config"), c.String("workdir"))
			cfg, err := moleman.LoadConfig(cfgPath)
			if err != nil {
				return err
			}

			for _, name := range moleman.AgentNames(cfg) {
				fmt.Println(name)
			}
			return nil
		},
	}
}

func explainCommand() *cli.Command {
	return &cli.Command{
		Name:      "explain",
		Usage:     "Print the resolved workflow",
		UsageText: "moleman explain [flags]",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "config", Usage: "config file path"},
			&cli.StringFlag{Name: "workdir", Usage: "working directory"},
		},
		Action: func(c *cli.Context) error {
			cfgPath := resolveConfigPath(c.String("config"), c.String("workdir"))
			cfg, err := moleman.LoadConfig(cfgPath)
			if err != nil {
				return err
			}

			return moleman.PrintWorkflow(cfg.Workflow)
		},
	}
}

func initCommand() *cli.Command {
	return &cli.Command{
		Name:      "init",
		Usage:     "Create an example config",
		UsageText: "moleman init [flags]",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "workdir", Usage: "working directory"},
			&cli.BoolFlag{Name: "force", Usage: "overwrite existing config"},
			&cli.StringFlag{Name: "config", Usage: "config file path"},
		},
		Action: func(c *cli.Context) error {
			cfgPath := resolveConfigPath(c.String("config"), c.String("workdir"))
			if err := moleman.Init(cfgPath, c.Bool("force")); err != nil {
				return err
			}
			log.Info("created", "path", cfgPath)
			return nil
		},
	}
}

func doctorCommand() *cli.Command {
	return &cli.Command{
		Name:      "doctor",
		Usage:     "Validate environment and config",
		UsageText: "moleman doctor [flags]",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "workdir", Usage: "working directory"},
			&cli.StringFlag{Name: "config", Usage: "config file path"},
		},
		Action: func(c *cli.Context) error {
			cfgPath := resolveConfigPath(c.String("config"), c.String("workdir"))
			if err := moleman.Doctor(cfgPath); err != nil {
				return err
			}
			log.Info("doctor ok")
			return nil
		},
	}
}

func versionCommand() *cli.Command {
	return &cli.Command{
		Name:      "version",
		Usage:     "Print version",
		UsageText: "moleman version",
		Action: func(c *cli.Context) error {
			fmt.Println(Version)
			return nil
		},
	}
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

func configureLogging() {
	log.SetReportTimestamp(true)
	log.SetTimeFormat("15:04:05")
	log.SetPrefix("moleman")
	log.SetColorProfile(termenv.TrueColor)

	styles := log.DefaultStyles()
	primary := lipgloss.Color("#c98301")
	styles.Levels[log.DebugLevel] = styles.Levels[log.DebugLevel].Foreground(lipgloss.Color("69")).Bold(true)
	styles.Levels[log.InfoLevel] = styles.Levels[log.InfoLevel].Foreground(primary).Bold(true)
	styles.Levels[log.WarnLevel] = styles.Levels[log.WarnLevel].Foreground(lipgloss.Color("220")).Bold(true)
	styles.Levels[log.ErrorLevel] = styles.Levels[log.ErrorLevel].Foreground(lipgloss.Color("196")).Bold(true)
	styles.Prefix = styles.Prefix.Foreground(primary).Bold(true)
	styles.Key = styles.Key.Foreground(lipgloss.Color("244"))
	log.SetStyles(styles)
}
