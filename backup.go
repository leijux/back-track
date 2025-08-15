package main

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	dataDirName = "data"
	workerCount = 4 // 并发worker数量
)

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "执行备份",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		outputPath, _ := cmd.Flags().GetString("output")
		return backup(configPath, outputPath)
	},
}

type Config struct {
	BackupPaths  []string `yaml:"backup_paths"`
	ExcludeDirs  []string `yaml:"exclude_dirs,omitempty"`
	ExcludeFiles []string `yaml:"exclude_files,omitempty"`
}

type FileMap map[string]string // key: 压缩包内路径, value: 原绝对路径

type fileTask struct {
	absPath string
	relPath string
}

var skippedFiles int64 // 跳过文件计数

func backup(configPath, outputPath string) error {
	cfg, configBytes, err := loadConfig(configPath)
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("创建输出文件失败: %w", err)
	}
	defer outFile.Close()

	zipWriter := zip.NewWriter(outFile)
	defer zipWriter.Close()

	var mu sync.Mutex // zip写入锁
	fileMap := make(FileMap)

	// 写配置文件到zip
	if err := writeZipFile(zipWriter, "backup_config.yaml", configBytes, &mu); err != nil {
		return err
	}

	// 先统计总文件数
	totalFiles, err := countTotalFiles(cfg)
	if err != nil {
		return err
	}
	log.Printf("共 %d 个文件待备份\n", totalFiles)

	// 初始化进度条
	bar := progressbar.Default(int64(totalFiles))

	tasks := make(chan fileTask, 1000)
	var wg sync.WaitGroup

	// 启动worker池
	for i := 0; i < workerCount; i++ {
		wg.Go(func() {
			for task := range tasks {
				if err := addFileToZip(zipWriter, task.absPath, task.relPath, &mu); err != nil {
					log.Printf("写入失败: %s → %s, 错误: %v", task.absPath, task.relPath, err)
					continue
				}
				mu.Lock()
				fileMap[task.relPath] = task.absPath
				mu.Unlock()
				bar.Add(1) // 进度+1
			}
		})
	}

	// 遍历路径并推送任务
	for _, path := range cfg.BackupPaths {
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("读取路径信息失败 (%s): %w", path, err)
		}

		if info.IsDir() {
			if err := walkDirAndPushTasks(cfg, path, tasks); err != nil {
				return err
			}
		} else {
			if shouldExcludeFile(cfg, filepath.Base(path)) {
				atomic.AddInt64(&skippedFiles, 1)
				continue
			}
			absPath, _ := filepath.Abs(path)
			tasks <- fileTask{
				absPath: absPath,
				relPath: filepath.Join(dataDirName, filepath.Base(path)),
			}
		}
	}

	close(tasks)
	wg.Wait()

	// 写 file_map.yaml
	mapBytes, _ := yaml.Marshal(fileMap)
	if err := writeZipFile(zipWriter, "file_map.yaml", mapBytes, &mu); err != nil {
		return err
	}

	fmt.Printf("\n备份完成: %s\n", outputPath)
	fmt.Printf("跳过文件: %d 个\n", skippedFiles)
	return nil
}

func walkDirAndPushTasks(cfg *Config, dirPath string, tasks chan<- fileTask) error {
	return filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && shouldExcludeDir(cfg, d.Name()) {
			return filepath.SkipDir
		}
		if !d.IsDir() && shouldExcludeFile(cfg, d.Name()) {
			atomic.AddInt64(&skippedFiles, 1)
			return nil
		}
		if !d.IsDir() {
			rel, _ := filepath.Rel(dirPath, path)
			relPath := filepath.Join(dataDirName, filepath.Base(dirPath), filepath.ToSlash(rel))
			absPath, _ := filepath.Abs(path)
			tasks <- fileTask{absPath: absPath, relPath: relPath}
		}
		return nil
	})
}

func countTotalFiles(cfg *Config) (int, error) {
	count := 0
	for _, path := range cfg.BackupPaths {
		info, err := os.Stat(path)
		if err != nil {
			return 0, err
		}
		if info.IsDir() {
			err = filepath.WalkDir(path, func(_ string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !d.IsDir() && !shouldExcludeFile(cfg, d.Name()) {
					count++
				} else if !d.IsDir() {
					atomic.AddInt64(&skippedFiles, 1)
				}
				return nil
			})
			if err != nil {
				return 0, err
			}
		} else {
			if !shouldExcludeFile(cfg, info.Name()) {
				count++
			} else {
				atomic.AddInt64(&skippedFiles, 1)
			}
		}
	}
	return count, nil
}

func shouldExcludeDir(cfg *Config, dirName string) bool {
	for _, exclude := range cfg.ExcludeDirs {
		if dirName == exclude {
			return true
		}
	}
	return false
}

func shouldExcludeFile(cfg *Config, fileName string) bool {
	for _, pattern := range cfg.ExcludeFiles {
		if match, _ := filepath.Match(pattern, fileName); match {
			return true
		}
	}
	return false
}

func addFileToZip(zipWriter *zip.Writer, filePath, relPath string, mu *sync.Mutex) error {
	srcFile, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	mu.Lock()
	defer mu.Unlock()
	writer, err := zipWriter.Create(relPath)
	if err != nil {
		return err
	}
	_, err = io.Copy(writer, srcFile)
	return err
}

func writeZipFile(zipWriter *zip.Writer, name string, data []byte, mu *sync.Mutex) error {
	mu.Lock()
	defer mu.Unlock()
	w, err := zipWriter.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func loadConfig(path string) (*Config, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, nil, err
	}
	return &cfg, data, nil
}
