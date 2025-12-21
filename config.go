package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"

	"github.com/goccy/go-yaml"
	"github.com/klauspost/compress/flate"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "处理配置",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

// configExportCmd 导出备份包中的配置
var configExportCmd = &cobra.Command{
	Use:   "export",
	Short: "从备份包导出配置",
	RunE: func(cmd *cobra.Command, args []string) error {
		inputPath, _ := cmd.Flags().GetString("input")
		outputPath, _ := cmd.Flags().GetString("output")
		return exportConfigFromBackup(inputPath, outputPath)
	},
}

// configImportCmd 导入配置到备份包
var configImportCmd = &cobra.Command{
	Use:   "import",
	Short: "导入配置到备份包",
	RunE: func(cmd *cobra.Command, args []string) error {
		inputPath, _ := cmd.Flags().GetString("input")
		configPath, _ := cmd.Flags().GetString("config")
		return importConfigToBackup(inputPath, configPath)
	},
}

func init() {
	configExportCmd.Flags().StringP("input", "i", "", "备份文件路径")
	configExportCmd.Flags().StringP("output", "o", backupConfigName, "导出的配置文件路径")
	configExportCmd.MarkFlagRequired("input")

	configImportCmd.Flags().StringP("input", "i", "", "备份文件路径")
	configImportCmd.Flags().StringP("config", "c", backupConfigName, "要导入的配置文件路径")
	configImportCmd.MarkFlagRequired("input")

	configCmd.AddCommand(configExportCmd)
	configCmd.AddCommand(configImportCmd)
	rootCmd.AddCommand(configCmd)
}

// exportConfigFromBackup 从备份包中导出配置
func exportConfigFromBackup(zipPath, outputPath string) error {
	// 打开备份文件
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("打开备份文件失败 (%s): %w", zipPath, err)
	}
	defer r.Close()

	r.RegisterDecompressor(zip.Deflate, func(r io.Reader) io.ReadCloser {
		return flate.NewReader(r)
	})

	// 查找配置文件
	var configData []byte
	for _, f := range r.File {
		if f.Name == "backupConfigName" {
			rc, err := f.Open()
			if err != nil {
				return fmt.Errorf("打开配置文件失败: %w", err)
			}
			defer rc.Close()

			configData, err = io.ReadAll(rc)
			if err != nil {
				return fmt.Errorf("读取配置文件失败: %w", err)
			}
			break
		}
	}

	if configData == nil {
		return fmt.Errorf("备份文件中未找到 %s", backupConfigName)
	}

	// 写入输出文件
	if err := os.WriteFile(outputPath, configData, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败 (%s): %w", outputPath, err)
	}

	fmt.Printf("配置已成功导出到: %s\n", outputPath)
	return nil
}

// importConfigToBackup 导入配置到备份包
func importConfigToBackup(zipPath, configPath string) error {
	// 读取配置文件
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败 (%s): %w", configPath, err)
	}

	// 验证配置文件格式
	var cfg Config
	if err := yaml.Unmarshal(configData, &cfg); err != nil {
		return fmt.Errorf("配置文件格式无效: %w", err)
	}

	// 创建临时文件
	tempZipPath := zipPath + ".tmp"
	if err := updateZipFile(zipPath, tempZipPath, configData); err != nil {
		return fmt.Errorf("更新备份文件失败: %w", err)
	}

	// 替换原文件
	if err := os.Rename(tempZipPath, zipPath); err != nil {
		return fmt.Errorf("替换备份文件失败: %w", err)
	}

	fmt.Printf("配置已成功导入到: %s\n", zipPath)
	return nil
}

// updateZipFile 更新zip文件中的配置文件
func updateZipFile(srcPath, dstPath string, newConfigData []byte) error {
	// 打开源zip文件
	srcZip, err := zip.OpenReader(srcPath)
	if err != nil {
		return fmt.Errorf("打开源备份文件失败: %w", err)
	}
	defer srcZip.Close()

	srcZip.RegisterDecompressor(zip.Deflate, func(r io.Reader) io.ReadCloser {
		return flate.NewReader(r)
	})

	// 创建目标zip文件
	dstFile, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("创建目标备份文件失败: %w", err)
	}
	defer dstFile.Close()

	dstZip := zip.NewWriter(dstFile)
	defer dstZip.Close()

	dstZip.RegisterCompressor(zip.Deflate, func(w io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(w, flate.BestCompression)
	})

	// 复制所有文件，替换配置文件
	configUpdated := false
	for _, f := range srcZip.File {
		if f.Name == backupConfigName {
			// 写入新的配置文件
			w, err := dstZip.Create(f.Name)
			if err != nil {
				return fmt.Errorf("创建zip条目失败: %w", err)
			}
			if _, err := w.Write(newConfigData); err != nil {
				return fmt.Errorf("写入配置文件失败: %w", err)
			}
			configUpdated = true
		} else {
			// 复制其他文件
			if err := copyZipFile(f, dstZip); err != nil {
				return fmt.Errorf("复制文件失败 (%s): %w", f.Name, err)
			}
		}
	}

	if !configUpdated {
		return fmt.Errorf("备份文件中未找到 %s", backupConfigName)
	}

	return nil
}

// copyZipFile 复制zip文件条目
func copyZipFile(src *zip.File, dst *zip.Writer) error {
	// 打开源文件
	rc, err := src.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	// 创建目标文件
	w, err := dst.Create(src.Name)
	if err != nil {
		return err
	}

	// 复制内容
	_, err = io.Copy(w, rc)
	return err
}
