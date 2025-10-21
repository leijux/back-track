package main

import (
	"os"
	"path/filepath"
	"testing"
)

func Test_restore(t *testing.T) {
	type args struct {
		zipPath     string
		fileDataMap map[string]string //还原后的文件和数据
	}
	tests := []struct {
		name       string
		args       args
		wantErr    bool
		wantExists bool
	}{
		{
			name: "restore1",
			args: args{
				zipPath: "testdata/output.zip",
				fileDataMap: map[string]string{
					"testdata/backup/data1/data5.txt": "test5",
				},
			},
			wantErr:    false,
			wantExists: true,
		},
		{
			name: "restore2",
			args: args{
				zipPath: "testdata/output.zip",
				fileDataMap: map[string]string{
					"testdata/backup/data1/data4.txt": "",
				},
			},
			wantErr:    false,
			wantExists: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			if err := restore(tt.args.zipPath, tempDir, false); (err != nil) != tt.wantErr {
				t.Errorf("restore() error = %v, wantErr %v", err, tt.wantErr)
			}
			if gotExists := filesMatchContent(tt.args.fileDataMap, tempDir); gotExists != tt.wantExists {
				t.Errorf("filesMatchContent() = %v, wantExists %v", gotExists, tt.wantExists)
			}
		})
	}
}

func filesMatchContent(expectedFiles map[string]string, baseDir string) bool {
	for relPath, expectedContent := range expectedFiles {
		fullPath := filepath.Join(baseDir, relPath)
		actualContent, err := os.ReadFile(fullPath)
		if err != nil {
			return false
		}
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			return false
		}
		if expectedContent != string(actualContent) {
			return false
		}
	}
	return true
}
