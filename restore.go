package main

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"
)

var restoreCmd = &cobra.Command{
	Use:     "restore",
	Short:   "执行还原",
	PreRunE: checkRoot,
	RunE: func(cmd *cobra.Command, args []string) error {
		inputPath, err := cmd.Flags().GetString("input")
		if err != nil {
			return err
		}
		if inputPath == "" {
			return fmt.Errorf("备份文件路径不能为空")
		}

		rootDir, _ := cmd.Flags().GetString("root-dir")
		backupBeforeRestore, _ := cmd.Flags().GetBool("backup-before-restore")
		noScripts, _ := cmd.Flags().GetBool("no-scripts")

		return restore(inputPath, rootDir, backupBeforeRestore, noScripts)
	},
}

func init() {
	restoreCmd.Flags().StringP("input", "i", "", "备份文件路径")
	restoreCmd.Flags().StringP("root-dir", "r", "/", "还原根目录")
	restoreCmd.Flags().BoolP("backup-before-restore", "b", false, "还原时是否备份，保留最近3个备份")
	restoreCmd.Flags().BoolP("no-script", "s", false, "还原时不执行脚本")
}

// restore 执行还原操作
func restore(zipPath string, rootDir string, backupBeforeRestore, noScripts bool) error {
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

	// 还原前备份
	if backupBeforeRestore {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("获取用户主目录r失败: %w", err)
		}

		restoreDirName := filepath.Join(homeDir, ".backup_restore")
		if err := os.MkdirAll(restoreDirName, 0755); err != nil {
			return fmt.Errorf("创建还原目录失败: %w", err)
		}

		backupPath := filepath.Join(restoreDirName, fmt.Sprintf("restore_%s.zip", time.Now().Format("20060102150405")))
		log.Printf("正在还原前备份当前文件，备份文件: %s", backupPath)

		configBytes, err := yaml.Marshal(cfg)
		if err != nil {
			return fmt.Errorf("序列化配置失败: %w", err)
		}
		if err := backup(cfg, configBytes, backupPath); err != nil {
			return fmt.Errorf("还原前备份失败: %w", err)
		}
		log.Printf("还原前备份完成: %s", backupPath)
		// 删除旧备份，只保留最新的3个
		cleanupOldBackups(restoreDirName, retainBackupCount)
	}

	// 设置根目录
	for k := range fileMap {
		fileMap[k] = filepath.Join(rootDir, fileMap[k])
	}

	// 执行还原前脚本
	if cfg.BeforeScript != "" && !noScripts {
		result, err := runCommand("sh", "-c", cfg.BeforeScript)
		if err != nil {
			log.Printf("执行还原前脚本失败: %v", err)
			return err
		}
		log.Printf("还原前脚本输出:\n%s", result)
	}

	// 执行还原后脚本
	if cfg.AfterScript != "" && !noScripts {
		defer func() {
			result, err := runCommand("sh", "-c", cfg.AfterScript)
			if err != nil {
				log.Printf("执行还原后脚本失败: %v", err)
				return
			}
			log.Printf("还原后脚本输出:\n%s", result)
		}()
	}

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

	log.Println("还原完成")
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

func cleanupOldBackups(dir string, maxBackups int) {
	files, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("读取备份目录失败 (%s): %v", dir, err)
		return
	}

	var backups []os.DirEntry
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".zip" {
			backups = append(backups, file)
		}
	}

	if len(backups) <= maxBackups {
		return
	}

	// 按文件名排序（假设文件名包含时间戳）
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Name() < backups[j].Name()
	})

	// 删除旧备份
	for i := 0; i < len(backups)-maxBackups; i++ {
		toDelete := filepath.Join(dir, backups[i].Name())
		if err := os.Remove(toDelete); err != nil {
			log.Printf("删除旧备份失败 (%s): %v", toDelete, err)
		} else {
			log.Printf("已删除旧备份: %s", toDelete)
		}
	}
}
