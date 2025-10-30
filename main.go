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
	"runtime/debug"
	"time"

	"github.com/spf13/cobra"
)

const (
	// 备份数据目录名称
	dataDirName = "data"

	// 保留最新的备份数量
	retainBackupCount = 3
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	debugInfo, _ := debug.ReadBuildInfo()
	rootCmd := &cobra.Command{
		Use:     "backtrack",
		Short:   "文件备份和还原工具",
		Version: debugInfo.Main.Version,
	}

	backupCmd.Flags().StringP("config", "c", "config.yaml", "配置文件路径")
	backupCmd.Flags().StringP("output", "o", fmt.Sprintf("backup_%s.zip", time.Now().Format("20060102150405")), "备份输出路径")

	restoreCmd.Flags().StringP("input", "i", "", "备份文件路径")
	restoreCmd.Flags().StringP("root-dir", "r", "/", "还原根目录")
	restoreCmd.Flags().BoolP("backup-before-restore", "b", false, "还原时是否备份，保留最近3个备份")
	restoreCmd.Flags().BoolP("no-script", "s", false, "还原时不执行脚本")

	rootCmd.AddCommand(backupCmd, restoreCmd)

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
