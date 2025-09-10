package main

import (
	"archive/zip"
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
			if err := backup(tt.args.configPath, outputPath, false); (err != nil) != tt.wantErr {
				t.Errorf("backup() error = %v, wantErr %v", err, tt.wantErr)
			}
			equal, err := zipFilesAreEqual(outputPath, "testdata/output.zip")
			if err != nil {
				t.Errorf("zipFilesAreEqual() error = %v", err)
			}
			if !equal {
				t.Errorf("backup() output zip file content mismatch: %v", err)
			}
		})
	}
}

// 对比两个zip压缩包文件是否内容相同
func zipFilesAreEqual(zip1, zip2 string) (bool, error) {
	f1, err := zip.OpenReader(zip1)
	if err != nil {
		return false, err
	}
	defer f1.Close()

	f2, err := zip.OpenReader(zip2)
	if err != nil {
		return false, err
	}
	defer f2.Close()

	if len(f1.File) != len(f2.File) {
		return false, nil
	}

	fMap := make(map[string]struct{}, len(f1.File))
	for i := range f1.File {
		fMap[f1.File[i].Name] = struct{}{}
	}

	for i := range f2.File {
		if _, exists := fMap[f2.File[i].Name]; !exists {
			return false, nil
		}
	}

	return true, nil
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
