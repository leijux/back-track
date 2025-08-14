// OEXPERIMENT=greenteagc
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/spf13/cobra"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	rootCmd := &cobra.Command{
		Use:   "backup-cli",
		Short: "文件备份和还原工具",
	}

	rootCmd.PersistentFlags().StringP("config", "c", "config.yaml", "配置文件路径")

	backupCmd.Flags().StringP("output", "o", fmt.Sprintf("backup_%s.zip", time.Now().Format("20060102150405")), "备份输出路径")

	restoreCmd.Flags().StringP("input", "i", "backup.zip", "备份文件路径")

	rootCmd.AddCommand(backupCmd, restoreCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatalln(err)
	}
}
