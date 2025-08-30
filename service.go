package main

import (
	"fmt"
	"log"
	"os/exec"
)

// 执行系统命令并返回结果
func runCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("命令执行失败: %v, 输出: %s", err, output)
	}
	return string(output), nil
}

// 暂停服务
func pauseService(serviceName string) error {
	_, err := runCommand("sudo", "systemctl", "stop", serviceName)
	if err != nil {
		return err
	}
	log.Printf("服务 %s 已暂停", serviceName)
	return nil
}

func pauseServices(serviceNames []string) error {
	for _, serviceName := range serviceNames {
		if err := pauseService(serviceName); err != nil {
			return err
		}
	}
	return nil
}

// 恢复服务
func resumeService(serviceName string) error {
	_, err := runCommand("sudo", "systemctl", "start", serviceName)
	if err != nil {
		return err
	}
	log.Printf("服务 %s 已恢复", serviceName)
	return nil
}

func resumeServices(serviceNames []string) error {
	for _, serviceName := range serviceNames {
		if err := resumeService(serviceName); err != nil {
			return err
		}
	}
	return nil
}
