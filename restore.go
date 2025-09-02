package main

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"
)

var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "执行还原",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := checkRoot(); err != nil {
			return err
		}
		inputPath, err := cmd.Flags().GetString("input")
		if err != nil {
			return err
		}
		if inputPath == "" {
			return fmt.Errorf("备份文件路径不能为空")
		}
		return restore(inputPath)
	},
}

// restore 执行还原操作
func restore(zipPath string) error {
	// 打开备份文件
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("无法打开备份文件 (%s): %w", zipPath, err)
	}
	defer r.Close()

	// 读取配置和文件映射
	cfg, fileMap, err := readBackupMetadata(r.File)
	if err != nil {
		return err
	}

	// 暂停服务
	pauseServices(cfg.ServiceNames)
	defer resumeServices(cfg.ServiceNames)

	// 获取需要还原的文件列表
	filesToRestore := getFilesToRestore(r.File)
	if len(filesToRestore) == 0 {
		return fmt.Errorf("备份文件中没有找到可还原的文件")
	}

	// 初始化进度条
	bar := progressbar.Default(int64(len(filesToRestore)), "正在还原")

	// 并发还原文件
	if err := restoreFilesConcurrently(filesToRestore, fileMap, bar); err != nil {
		return err
	}

	log.Println("\n还原完成")
	return nil
}

// readBackupMetadata 读取备份文件的元数据（配置和文件映射）
func readBackupMetadata(files []*zip.File) (*Config, FileMap, error) {
	var cfg Config
	var fileMap FileMap

	if err := readYAMLFromZip(files, "backup_config.yaml", &cfg); err != nil {
		return nil, nil, fmt.Errorf("读取 backup_config.yaml 失败: %w", err)
	}

	if err := readYAMLFromZip(files, "file_map.yaml", &fileMap); err != nil {
		return nil, nil, fmt.Errorf("读取 file_map.yaml 失败: %w", err)
	}

	if len(fileMap) == 0 {
		return nil, nil, fmt.Errorf("备份文件缺少 file_map.yaml，无法还原")
	}

	return &cfg, fileMap, nil
}

// getFilesToRestore 获取需要还原的文件列表（排除元数据文件）
func getFilesToRestore(files []*zip.File) []*zip.File {
	filesToRestore := make([]*zip.File, 0, len(files))
	for _, f := range files {
		if f.Name != "backup_config.yaml" && f.Name != "file_map.yaml" {
			filesToRestore = append(filesToRestore, f)
		}
	}
	return filesToRestore
}

// restoreFilesConcurrently 并发还原文件
func restoreFilesConcurrently(filesToRestore []*zip.File, fileMap FileMap, bar *progressbar.ProgressBar) error {
	g := new(errgroup.Group)
	sem := make(chan struct{}, runtime.NumCPU()) // 限制并发量

	for _, f := range filesToRestore {
		f := f // 避免闭包变量引用问题
		targetPath, ok := fileMap[f.Name]
		if !ok {
			log.Printf("跳过未知文件: %s", f.Name)
			continue
		}

		g.Go(func() error {
			sem <- struct{}{}        // 获取信号量
			defer func() { <-sem }() // 释放信号量

			if err := extractFile(f, targetPath); err != nil {
				return fmt.Errorf("还原文件 %s 失败: %w", targetPath, err)
			}

			bar.Add(1)
			return nil
		})
	}

	return g.Wait()
}

// readYAMLFromZip 从zip文件中读取并解析YAML数据
func readYAMLFromZip(files []*zip.File, filename string, out any) error {
	for _, f := range files {
		if f.Name == filename {
			rc, err := f.Open()
			if err != nil {
				return fmt.Errorf("打开zip文件 %s 失败: %w", filename, err)
			}
			defer rc.Close()

			data, err := io.ReadAll(rc)
			if err != nil {
				return fmt.Errorf("读取zip文件 %s 内容失败: %w", filename, err)
			}

			if err := yaml.Unmarshal(data, out); err != nil {
				return fmt.Errorf("解析YAML文件 %s 失败: %w", filename, err)
			}

			return nil
		}
	}
	return fmt.Errorf("未找到文件: %s", filename)
}

// extractFile 从zip文件中提取文件到目标路径
func extractFile(f *zip.File, targetPath string) error {
	// 打开zip文件条目
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("打开zip条目 %s 失败: %w", f.Name, err)
	}
	defer rc.Close()

	// 创建目标目录
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("创建目录 %s 失败: %w", filepath.Dir(targetPath), err)
	}

	// 创建目标文件
	outFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("创建文件 %s 失败: %w", targetPath, err)
	}
	defer outFile.Close()

	// 复制文件内容
	if _, err := io.Copy(outFile, rc); err != nil {
		return fmt.Errorf("复制文件内容 %s → %s 失败: %w", f.Name, targetPath, err)
	}

	return nil
}
