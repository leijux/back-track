package main

import (
	"fmt"
	"log"
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

// pauseService 暂停单个服务
func pauseService(serviceName string) error {
	_, err := runCommand("sudo", "systemctl", "stop", serviceName)
	if err != nil {
		return fmt.Errorf("暂停服务 %s 失败: %w", serviceName, err)
	}
	log.Printf("服务 %s 已暂停", serviceName)
	return nil
}

// pauseServices 批量暂停服务
func pauseServices(serviceNames []string) error {
	for _, serviceName := range serviceNames {
		if err := pauseService(serviceName); err != nil {
			return fmt.Errorf("批量暂停服务失败: %w", err)
		}
	}
	return nil
}

// resumeService 恢复单个服务
func resumeService(serviceName string) error {
	_, err := runCommand("sudo", "systemctl", "start", serviceName)
	if err != nil {
		return fmt.Errorf("恢复服务 %s 失败: %w", serviceName, err)
	}
	log.Printf("服务 %s 已恢复", serviceName)
	return nil
}

// resumeServices 批量恢复服务
func resumeServices(serviceNames []string) error {
	for _, serviceName := range serviceNames {
		if err := resumeService(serviceName); err != nil {
			return fmt.Errorf("批量恢复服务失败: %w", err)
		}
	}
	return nil
}
