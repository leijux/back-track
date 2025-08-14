package main

import (
	"archive/zip"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "执行备份",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, err := cmd.Flags().GetString("config")
		if err != nil {
			return err
		}

		outputPath, err := cmd.Flags().GetString("output")
		if err != nil {
			return err
		}

		return backup(configPath, outputPath)
	},
}

type Config struct {
	BackupPaths  []string `yaml:"backup_paths"`
	ExcludeDirs  []string `yaml:"exclude_dirs,omitempty"`
	ExcludeFiles []string `yaml:"exclude_files,omitempty"`
}

type FileMap map[string]string // key: 压缩包内路径, value: 原绝对路径

func backup(configPath, outputPath string) error {
	cfg, configBytes, err := loadConfig(configPath)
	if err != nil {
		return err
	}

	outFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	zipWriter := zip.NewWriter(outFile)
	defer zipWriter.Close()

	// 保存配置
	configFile, _ := zipWriter.Create("backup_config.yaml")
	configFile.Write(configBytes)

	fileMap := make(FileMap)

	for _, p := range cfg.BackupPaths {
		info, err := os.Stat(p)
		if err != nil {
			return err
		}

		if info.IsDir() {
			// 目录备份
			err = filepath.Walk(p, func(path string, fi os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				if fi.IsDir() {
					// 检查是否排除该文件夹
					for _, exclude := range cfg.ExcludeDirs {
						if fi.Name() == exclude {
							return filepath.SkipDir
						}
					}
					return nil
				}

				// 检查是否排除文件
				for _, pattern := range cfg.ExcludeFiles {
					match, _ := filepath.Match(pattern, fi.Name())
					if match {
						log.Println("跳过文件:", p)
						return nil
					}
				}

				relPath := filepath.Join("data", filepath.Base(p), filepath.ToSlash(path[len(p):]))
				absPath, err := filepath.Abs(path)
				if err != nil {
					return err
				}
				fileMap[relPath] = absPath
				log.Println("添加文件:", path, "→", relPath)
				return addFileToZip(zipWriter, path, relPath)
			})
			if err != nil {
				return err
			}
		} else {
			// 单文件备份
			for _, pattern := range cfg.ExcludeFiles {
				match, _ := filepath.Match(pattern, info.Name())
				if match {
					log.Println("跳过文件:", p)
					continue
				}
			}

			relPath := filepath.Join("data", filepath.Base(p))
			absPath, err := filepath.Abs(p)
			if err != nil {
				return err
			}
			fileMap[relPath] = absPath
			log.Println("添加文件:", p, "→", relPath)
			if err := addFileToZip(zipWriter, p, relPath); err != nil {
				return err
			}
		}
	}

	// 保存 file_map.yaml
	mapBytes, _ := yaml.Marshal(fileMap)
	mapFile, _ := zipWriter.Create("file_map.yaml")
	mapFile.Write(mapBytes)

	log.Println("备份完成:", outputPath)
	return nil
}

func addFileToZip(zipWriter *zip.Writer, filePath, relPath string) error {
	writer, err := zipWriter.Create(relPath)
	if err != nil {
		return err
	}
	srcFile, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	_, err = io.Copy(writer, srcFile)
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
