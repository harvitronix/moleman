package moleman

import (
	"bytes"
	"os/exec"
	"strings"
)

func LoadGitData(workdir string) GitData {
	return GitData{
		Diff:   runGitCommand(workdir, "diff"),
		Status: runGitCommand(workdir, "status", "--porcelain"),
		Branch: runGitCommand(workdir, "rev-parse", "--abbrev-ref", "HEAD"),
		Root:   runGitCommand(workdir, "rev-parse", "--show-toplevel"),
	}
}

func runGitCommand(workdir string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = workdir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(out.String())
}
