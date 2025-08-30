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

func restore(zipPath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("无法打开备份文件: %w", err)
	}
	defer r.Close()

	var cfg Config
	var fileMap FileMap

	// 读取配置文件和文件映射
	if err := readYAMLFromZip(r.File, "backup_config.yaml", &cfg); err != nil {
		return fmt.Errorf("读取 backup_config.yaml 失败: %w", err)
	}
	if err := readYAMLFromZip(r.File, "file_map.yaml", &fileMap); err != nil {
		return fmt.Errorf("读取 file_map.yaml 失败: %w", err)
	}
	if len(fileMap) == 0 {
		return fmt.Errorf("备份文件缺少 file_map.yaml，无法还原")
	}

	// 暂停服务
	if err := pauseServices(cfg.ServiceNames); err != nil {
		return fmt.Errorf("暂停服务失败: %w", err)
	}

	defer func() {
		if err := resumeServices(cfg.ServiceNames); err != nil {
			log.Printf("恢复服务失败: %v", err)
		}
	}()

	// 进度条
	filesToRestore := []*zip.File{}
	for _, f := range r.File {
		if f.Name != "backup_config.yaml" && f.Name != "file_map.yaml" {
			filesToRestore = append(filesToRestore, f)
		}
	}
	bar := progressbar.Default(int64(len(filesToRestore)), "正在还原")

	// 并发还原文件
	g := new(errgroup.Group)
	sem := make(chan struct{}, runtime.NumCPU()) // 限制并发量

	for _, f := range filesToRestore {
		f := f // 避免闭包变量引用问题
		targetPath, ok := fileMap[f.Name]
		if !ok {
			log.Printf("跳过未知文件: %s\n", f.Name)
			continue
		}

		g.Go(func() error {
			sem <- struct{}{} // 占位
			defer func() { <-sem }()

			if err := extractFile(f, targetPath); err != nil {
				return fmt.Errorf("还原文件 %s 失败: %w", targetPath, err)
			}

			bar.Add(1)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	log.Println("\n还原完成")
	return nil
}

func readYAMLFromZip(files []*zip.File, filename string, out any) error {
	for _, f := range files {
		if f.Name == filename {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()

			data, err := io.ReadAll(rc)
			if err != nil {
				return err
			}
			return yaml.Unmarshal(data, out)
		}
	}
	return fmt.Errorf("未找到文件: %s", filename)
}

func extractFile(f *zip.File, targetPath string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return err
	}

	outFile, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, rc)
	return err
}
