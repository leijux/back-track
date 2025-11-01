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
	"log"
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "backtrack",
	Short: "文件备份和还原工具",
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	debugInfo, _ := debug.ReadBuildInfo()
	rootCmd.Version = debugInfo.Main.Version
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
