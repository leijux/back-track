// backtrack - 文件备份和还原工具
//
// 功能特性：
// - 支持多路径备份和还原
// - 支持文件/目录排除规则
// - 支持服务暂停/恢复（systemd服务）
// - 并发处理提高性能
// - 进度条显示
// - 压缩备份文件
//
// 使用方法：
//
//	备份: backtrack backup -c config.yaml -o backup.zip
//	还原: backtrack restore -i backup.zip
//
// 配置文件格式参考 config.yaml
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime/debug"

	"github.com/spf13/cobra"
)

const (
	// 备份数据目录名称
	dataDirName = "data"

	// 保留最新的备份数量
	retainBackupCount = 3
)

var rootCmd = &cobra.Command{
	Use:   "backtrack",
	Short: "文件备份和还原工具",
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	debugInfo, _ := debug.ReadBuildInfo()
	rootCmd.Version = debugInfo.Main.Version

	rootCmd.AddCommand(backupCmd, restoreCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

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
