package main

import (
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
				outputPath: "testdata/output.zip",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := backup(tt.args.configPath, tt.args.outputPath); (err != nil) != tt.wantErr {
				t.Errorf("backup() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
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
					"./testdata/backup/data2.txt",
				},
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
