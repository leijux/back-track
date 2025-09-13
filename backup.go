package main

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/klauspost/compress/flate"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "执行备份",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := checkRoot(); err != nil {
			return err
		}

		configPath, _ := cmd.Flags().GetString("config")
		outputPath, _ := cmd.Flags().GetString("output")
		noRestart, _ := cmd.Flags().GetBool("no-restart")

		cfg, configBytes, err := loadConfig(configPath)
		if err != nil {
			return fmt.Errorf("加载配置失败: %w", err)
		}

		return backup(cfg, configBytes, outputPath, noRestart)
	},
}

type Config struct {
	BackupPaths  []string `yaml:"backup_paths"`
	ExcludeDirs  []string `yaml:"exclude_dirs,omitempty"`
	ExcludeFiles []string `yaml:"exclude_files,omitempty"`
	ServiceNames []string `yaml:"service_names,omitempty"`
}

type FileMap map[string]string // key: 压缩包内路径, value: 原绝对路径

type fileTask struct {
	absPath string
	relPath string
}

var (
	skippedFiles atomic.Int64 // 跳过文件计数
	skippedDirs  atomic.Int64 // 跳过文件夹计数
)

// backup 执行备份操作
func backup(cfg *Config, configBytes []byte, outputPath string, noRestart bool) error {
	// 暂停服务
	if !noRestart {
		pauseServices(cfg.ServiceNames)
		defer resumeServices(cfg.ServiceNames)
	}

	// 创建备份文件
	zipWriter, outFile, err := createBackupFile(outputPath)
	if err != nil {
		return err
	}
	defer outFile.Close()
	defer zipWriter.Close()

	// 配置压缩算法
	configureCompression(zipWriter)

	var mu sync.Mutex
	fileMap := make(FileMap)

	// 写入配置文件到备份包
	if err := writeConfigToZip(zipWriter, configBytes, &mu); err != nil {
		return err
	}

	// 处理文件备份
	if err := processBackupFiles(cfg, zipWriter, &mu, fileMap); err != nil {
		return err
	}

	// 写入文件映射到备份包
	if err := writeFileMapToZip(zipWriter, fileMap, &mu); err != nil {
		return err
	}

	logBackupCompletion(outputPath)
	return nil
}

// createBackupFile 创建备份文件并返回zip writer
func createBackupFile(outputPath string) (*zip.Writer, *os.File, error) {
	dir := filepath.Dir(outputPath)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, nil, fmt.Errorf("创建输出目录失败: %w", err)
	}

	outFile, err := os.Create(outputPath)
	if err != nil {
		return nil, nil, fmt.Errorf("创建输出文件失败: %w", err)
	}

	return zip.NewWriter(outFile), outFile, nil
}

// configureCompression 配置压缩算法
func configureCompression(zipWriter *zip.Writer) {
	zipWriter.RegisterCompressor(zip.Deflate, func(w io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(w, flate.BestCompression)
	})
}

// writeConfigToZip 将配置文件写入zip
func writeConfigToZip(zipWriter *zip.Writer, configBytes []byte, mu *sync.Mutex) error {
	return writeZipFile(zipWriter, "backup_config.yaml", configBytes, mu)
}

// processBackupFiles 处理文件备份过程
func processBackupFiles(cfg *Config, zipWriter *zip.Writer, mu *sync.Mutex, fileMap FileMap) error {
	// 统计总文件数
	totalFiles, err := countTotalFiles(cfg)
	if err != nil {
		return err
	}
	fmt.Printf("共 %d 个文件待备份\n", totalFiles)

	// 初始化进度条
	bar := progressbar.Default(int64(totalFiles))

	// 创建任务通道和worker池
	tasks := make(chan fileTask, 1000)
	var wg sync.WaitGroup

	// 启动worker处理文件
	startWorkers(zipWriter, mu, fileMap, bar, tasks, &wg, runtime.NumCPU())

	// 遍历备份路径并分发任务
	if err := processBackupPaths(cfg, tasks); err != nil {
		close(tasks)
		return err
	}

	// 等待所有任务完成
	close(tasks)
	wg.Wait()

	return nil
}

// startWorkers 启动worker协程处理文件任务
func startWorkers(zipWriter *zip.Writer, mu *sync.Mutex, fileMap FileMap,
	bar *progressbar.ProgressBar, tasks chan fileTask, wg *sync.WaitGroup, workerCount int) {

	for i := 0; i < workerCount; i++ {
		wg.Go(func() {
			processFileTasks(zipWriter, mu, fileMap, bar, tasks)
		})
	}
}

// processFileTasks 处理文件任务队列
func processFileTasks(zipWriter *zip.Writer, mu *sync.Mutex, fileMap FileMap,
	bar *progressbar.ProgressBar, tasks chan fileTask) {

	for task := range tasks {
		if err := processSingleFile(zipWriter, task, mu, fileMap); err != nil {
			log.Printf("文件处理失败: %s → %s, 错误: %v", task.absPath, task.relPath, err)
			continue
		}
		bar.Add(1)
	}
}

// processSingleFile 处理单个文件
func processSingleFile(zipWriter *zip.Writer, task fileTask, mu *sync.Mutex, fileMap FileMap) error {
	if err := addFileToZip(zipWriter, task.absPath, task.relPath, mu); err != nil {
		return err
	}

	mu.Lock()
	fileMap[task.relPath] = task.absPath
	mu.Unlock()

	return nil
}

// processBackupPaths 处理所有备份路径
func processBackupPaths(cfg *Config, tasks chan<- fileTask) error {
	for _, path := range cfg.BackupPaths {
		if err := processSinglePath(cfg, path, tasks); err != nil {
			return err
		}
	}
	return nil
}

// processSinglePath 处理单个备份路径
func processSinglePath(cfg *Config, path string, tasks chan<- fileTask) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("读取路径信息失败 (%s): %w", path, err)
	}

	if info.IsDir() {
		return walkDirAndPushTasks(cfg, path, tasks)
	} else {
		return processSingleFileTask(cfg, path, info, tasks)
	}
}

// processSingleFileTask 处理单个文件任务
func processSingleFileTask(cfg *Config, path string, info os.FileInfo, tasks chan<- fileTask) error {
	if shouldExcludeFile(cfg, info.Name()) {
		skippedFiles.Add(1)
		return nil
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("获取绝对路径失败 (%s): %w", path, err)
	}

	tasks <- fileTask{
		absPath: absPath,
		relPath: filepath.Join(dataDirName, filepath.Base(path)),
	}
	return nil
}

// writeFileMapToZip 将文件映射写入zip
func writeFileMapToZip(zipWriter *zip.Writer, fileMap FileMap, mu *sync.Mutex) error {
	mapBytes, err := yaml.Marshal(fileMap)
	if err != nil {
		return fmt.Errorf("序列化文件映射失败: %w", err)
	}
	return writeZipFile(zipWriter, "file_map.yaml", mapBytes, mu)
}

// logBackupCompletion 记录备份完成信息
func logBackupCompletion(outputPath string) {
	fmt.Printf("\n备份完成: %s ", outputPath)
	fmt.Printf("跳过 %d个文件 %d个文件夹\n", skippedFiles.Load(), skippedDirs.Load())
}

// walkDirAndPushTasks 遍历目录并将文件任务推送到通道
func walkDirAndPushTasks(cfg *Config, dirPath string, tasks chan<- fileTask) error {
	return filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("遍历目录失败 (%s): %w", path, err)
		}

		// 排除目录
		if d.IsDir() && shouldExcludeDir(cfg, d.Name()) {
			skippedDirs.Add(1)
			return filepath.SkipDir
		}

		// 排除文件
		if !d.IsDir() && shouldExcludeFile(cfg, d.Name()) {
			skippedFiles.Add(1)
			return nil
		}

		// 处理文件
		if !d.IsDir() {
			rel, err := filepath.Rel(dirPath, path)
			if err != nil {
				return fmt.Errorf("获取相对路径失败 (%s): %w", path, err)
			}

			absPath, err := filepath.Abs(path)
			if err != nil {
				return fmt.Errorf("获取绝对路径失败 (%s): %w", path, err)
			}

			relPath := filepath.Join(dataDirName, filepath.Base(dirPath), filepath.ToSlash(rel))
			tasks <- fileTask{absPath: absPath, relPath: relPath}
		}
		return nil
	})
}

// countTotalFiles 统计需要备份的文件总数
func countTotalFiles(cfg *Config) (int, error) {
	count := 0
	for _, path := range cfg.BackupPaths {
		info, err := os.Stat(path)
		if err != nil {
			return 0, fmt.Errorf("读取路径信息失败 (%s): %w", path, err)
		}

		if info.IsDir() {
			err = filepath.WalkDir(path, func(filePath string, d os.DirEntry, err error) error {
				if err != nil {
					return fmt.Errorf("遍历目录失败 (%s): %w", filePath, err)
				}

				// 排除目录
				if d.IsDir() && shouldExcludeDir(cfg, d.Name()) {
					skippedDirs.Add(1)
					return filepath.SkipDir
				}

				// 排除文件
				if !d.IsDir() && shouldExcludeFile(cfg, d.Name()) {
					skippedFiles.Add(1)
					return nil
				}

				// 统计文件
				if !d.IsDir() {
					count++
				}
				return nil
			})

			if err != nil {
				return 0, err
			}
		} else {
			// 处理单个文件
			if !shouldExcludeFile(cfg, info.Name()) {
				count++
			} else {
				skippedFiles.Add(1)
			}
		}
	}
	return count, nil
}

// shouldExcludeDir 检查目录是否应该被排除
func shouldExcludeDir(cfg *Config, dirName string) bool {
	for _, exclude := range cfg.ExcludeDirs {
		if dirName == exclude {
			return true
		}
	}
	return false
}

// shouldExcludeFile 检查文件是否应该被排除（支持通配符匹配）
func shouldExcludeFile(cfg *Config, fileName string) bool {
	for _, pattern := range cfg.ExcludeFiles {
		if match, _ := filepath.Match(pattern, fileName); match {
			return true
		}
	}
	return false
}

// addFileToZip 将文件添加到zip压缩包
func addFileToZip(zipWriter *zip.Writer, filePath, relPath string, mu *sync.Mutex) error {
	srcFile, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("打开文件失败 (%s): %w", filePath, err)
	}
	defer srcFile.Close()

	mu.Lock()
	defer mu.Unlock()

	writer, err := zipWriter.Create(relPath)
	if err != nil {
		return fmt.Errorf("创建zip条目失败 (%s): %w", relPath, err)
	}

	_, err = io.Copy(writer, srcFile)
	if err != nil {
		return fmt.Errorf("复制文件内容失败 (%s): %w", filePath, err)
	}

	return nil
}

// writeZipFile 将数据写入zip文件（线程安全）
func writeZipFile(zipWriter *zip.Writer, name string, data []byte, mu *sync.Mutex) error {
	mu.Lock()
	defer mu.Unlock()

	w, err := zipWriter.Create(name)
	if err != nil {
		return fmt.Errorf("创建zip文件失败 (%s): %w", name, err)
	}

	_, err = w.Write(data)
	if err != nil {
		return fmt.Errorf("写入zip文件失败 (%s): %w", name, err)
	}

	return nil
}

// loadConfig 加载并解析YAML配置文件
func loadConfig(path string) (*Config, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("读取配置文件失败 (%s): %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, nil, fmt.Errorf("解析YAML配置失败 (%s): %w", path, err)
	}

	return &cfg, data, nil
}
