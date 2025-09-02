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
	return nil
}

// pauseServices 批量暂停服务
func pauseServices(serviceNames []string) {
	for _, serviceName := range serviceNames {
		if err := pauseService(serviceName); err != nil {
			log.Printf("暂停服务 %s 失败: %v", serviceName, err)
			continue
		}
		log.Printf("服务 %s 已暂停", serviceName)
	}
}

// resumeService 恢复单个服务
func resumeService(serviceName string) error {
	_, err := runCommand("sudo", "systemctl", "start", serviceName)
	if err != nil {
		return fmt.Errorf("恢复服务 %s 失败: %w", serviceName, err)
	}
	return nil
}

// resumeServices 批量恢复服务
func resumeServices(serviceNames []string) {
	for _, serviceName := range serviceNames {
		if err := resumeService(serviceName); err != nil {
			log.Printf("恢复服务 %s 失败: %v", serviceName, err)
			continue
		}
		log.Printf("服务 %s 已恢复", serviceName)
	}
}
