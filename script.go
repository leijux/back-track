package main

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/goccy/go-yaml"
	"github.com/klauspost/compress/flate"
	"github.com/spf13/cobra"
)

var scriptCmd = &cobra.Command{
	Use:   "script",
	Short: "执行前置或后置脚本",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		scriptType, _ := cmd.Flags().GetString("type")
		configPath, _ := cmd.Flags().GetString("config")
		inputPath, _ := cmd.Flags().GetString("input")

		if configPath == "" && inputPath == "" {
			return fmt.Errorf("必须提供 config 或 input 参数")
		}
		if configPath != "" && inputPath != "" {
			return fmt.Errorf("不能同时提供 config 和 input 参数")
		}
		if scriptType != "before" && scriptType != "after" {
			return fmt.Errorf("type 必须是 'before' 或 'after'")
		}

		return checkRoot(cmd, args)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		scriptType, _ := cmd.Flags().GetString("type")
		configPath, _ := cmd.Flags().GetString("config")
		inputPath, _ := cmd.Flags().GetString("input")

		var (
			scriptContent string
			err           error
		)

		if configPath != "" {
			scriptContent, err = getScriptFromConfig(configPath, scriptType)
		} else {
			scriptContent, err = getScriptFromBackup(inputPath, scriptType)
		}
		if err != nil {
			return err
		}

		if scriptContent == "" {
			log.Printf("未找到 %s 脚本", scriptType)
			return nil
		}

		return runScript(scriptContent, scriptType)
	},
}

func init() {
	scriptCmd.Flags().StringP("type", "t", "before", "脚本类型 (before|after)")
	scriptCmd.Flags().StringP("config", "c", "", "YAML配置文件路径")
	scriptCmd.Flags().StringP("input", "i", "", "备份文件路径")
	scriptCmd.MarkFlagRequired("type")

	rootCmd.AddCommand(scriptCmd)
}

// getScriptFromConfig 从YAML配置文件中读取脚本
func getScriptFromConfig(configPath, scriptType string) (string, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("读取配置文件失败 (%s): %w", configPath, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("解析YAML配置失败 (%s): %w", configPath, err)
	}

	if scriptType == "before" {
		return cfg.BeforeScript, nil
	}
	return cfg.AfterScript, nil
}

// getScriptFromBackup 从备份包中读取脚本
func getScriptFromBackup(zipPath, scriptType string) (string, error) {
	// 打开备份文件
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("打开备份文件失败 (%s): %w", zipPath, err)
	}
	defer r.Close()

	r.RegisterDecompressor(zip.Deflate, func(r io.Reader) io.ReadCloser {
		return flate.NewReader(r)
	})

	// 读取配置
	var cfg Config
	if err := readYAMLFromZip(r.File, backupConfigName, &cfg); err != nil {
		return "", fmt.Errorf("从备份包读取配置失败: %w", err)
	}

	if scriptType == "before" {
		return cfg.BeforeScript, nil
	}
	return cfg.AfterScript, nil
}

// runScript 执行脚本
func runScript(scriptContent, scriptType string) error {
	result, err := runCommand("sh", "-c", scriptContent)
	if err != nil {
		return fmt.Errorf("执行 %s 脚本失败: %w", scriptType, err)
	}

	log.Printf("%s 脚本输出:\n%s", scriptType, result)
	log.Printf("%s 脚本执行完成", scriptType)

	return nil
}
