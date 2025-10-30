package main

import (
	"fmt"
	"os/exec"
)

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
