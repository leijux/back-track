package main

import (
	"os"
	"runtime/pprof"
	"testing"
)

func TestMain(m *testing.M) {
	// 在测试前设置性能分析
	f, err := os.Create("default.pgo")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	// 开始性能分析
	err = pprof.StartCPUProfile(f)
	if err != nil {
		panic(err)
	}

	// 运行测试
	code := m.Run()

	// 停止性能分析
	pprof.StopCPUProfile()

	os.Exit(code)
}
