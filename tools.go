package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

func checkRoot(cmd *cobra.Command, args []string) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("请以root权限运行")
	}
	return nil
}

// runCommand 执行系统命令并返回结果
func runCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("命令执行失败 (%s %v): %w, 输出: %s",
			name, args, err, string(output))
	}
	return string(output), nil
}

func newProgressBar(filesToRestoreCount int64, quiet bool, description string) *progressbar.ProgressBar {
	if quiet {
		return progressbar.DefaultSilent(filesToRestoreCount, description)
	}
	return progressbar.Default(filesToRestoreCount, description)
}
