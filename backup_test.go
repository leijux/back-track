package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func Test_backup(t *testing.T) {
	type args struct {
		configPath string
		outputPath string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "backup",
			args: args{
				configPath: "testdata/config.yaml",
				outputPath: "output.zip",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputPath := filepath.Join(t.TempDir(), tt.args.outputPath)
			if err := backup(tt.args.configPath, outputPath); (err != nil) != tt.wantErr {
				t.Errorf("backup() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !compareFileHashes(outputPath, "testdata/output.zip") {
				t.Errorf("backup() output file hash mismatch")
			}
		})
	}
}

func compareFileHashes(file1, file2 string) bool {
	hash1, err := computeFileHash(file1)
	if err != nil {
		return false
	}
	hash2, err := computeFileHash(file2)
	if err != nil {
		return false
	}

	return hash1 == hash2
}

func computeFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("计算文件哈希失败: %w", err)
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func Test_loadConfig(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name    string
		args    args
		want    *Config
		wantErr bool
	}{
		{
			name: "load config file",
			args: args{
				path: "testdata/config.yaml",
			},
			want: &Config{
				BackupPaths: []string{
					"./testdata/backup/data1",
					"./testdata/backup/data3.txt",
				},
				ExcludeDirs:  []string{"data2"},
				ExcludeFiles: []string{"data4.txt"},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := loadConfig(tt.args.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("loadConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("loadConfig() got = %v, want %v", got, tt.want)
			}
		})
	}
}
