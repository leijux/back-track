package main

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
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
		return restore(inputPath)
	},
}

func restore(zipPath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	var cfg Config
	var fileMap FileMap

	// 读取配置和文件映射
	for _, f := range r.File {
		if f.Name == "backup_config.yaml" {
			rc, _ := f.Open()
			data, _ := io.ReadAll(rc)
			rc.Close()
			yaml.Unmarshal(data, &cfg)
		}
		if f.Name == "file_map.yaml" {
			rc, _ := f.Open()
			data, _ := io.ReadAll(rc)
			rc.Close()
			yaml.Unmarshal(data, &fileMap)
		}
	}

	if len(fileMap) == 0 {
		return fmt.Errorf("备份文件缺少 file_map.yaml，无法还原")
	}

	// 还原文件
	for _, f := range r.File {
		if f.Name == "backup_config.yaml" || f.Name == "file_map.yaml" {
			continue
		}

		targetPath, ok := fileMap[f.Name]
		if !ok {
			log.Println("跳过未知文件:", f.Name)
			continue
		}

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
		if _, err := io.Copy(outFile, rc); err != nil {
			return err
		}
		outFile.Close()

		log.Println("还原文件:", targetPath)
	}

	log.Println("还原完成")
	return nil
}
