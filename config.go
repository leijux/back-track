package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/goccy/go-yaml"
	"github.com/klauspost/compress/flate"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "处理配置",
	RunE: func(cmd *cobra.Command, args []string) error {
		backupConfigPath, _ := cmd.Flags().GetString("backup-config")
		viewConfig, _ := cmd.Flags().GetString("view-config")
		content, err := readFile(backupConfigPath, viewConfig)
		if err != nil {
			return err
		}

		p := tea.NewProgram(
			model{title: viewConfig, content: string(content)},
			tea.WithAltScreen(),
			tea.WithMouseCellMotion(),
		)

		if _, err := p.Run(); err != nil {
			cmd.SilenceUsage = true
			return err
		}

		return nil
	},
}

// configExportCmd 导出备份包中的配置
var configExportCmd = &cobra.Command{
	Use:   "export",
	Short: "从备份包导出配置",
	RunE: func(cmd *cobra.Command, args []string) error {
		backupConfigPath, _ := cmd.Flags().GetString("backup-config")
		exportConfig, _ := cmd.Flags().GetString("config")
		outputPath, _ := cmd.Flags().GetString("output")
		if outputPath == "" {
			outputPath = filepath.Base(exportConfig)
		}

		return exportConfigFromBackup(backupConfigPath, exportConfig, outputPath)
	},
}

// configImportCmd 导入配置到备份包
var configImportCmd = &cobra.Command{
	Use:   "import",
	Short: "导入配置到备份包",
	RunE: func(cmd *cobra.Command, args []string) error {
		backupConfigPath, _ := cmd.Flags().GetString("backup-config")
		importConfig, _ := cmd.Flags().GetString("config")
		configPath, _ := cmd.Flags().GetString("import")
		force, _ := cmd.Flags().GetBool("force")

		return importConfigToBackup(backupConfigPath, importConfig, configPath, force)
	},
}

func init() {
	configCmd.Flags().StringP("view-config", "v", backupConfigName, fmt.Sprintf("要查看的配置文件名称(%s, %s)", backupConfigName, backupFileMapName))
	configCmd.PersistentFlags().StringP("backup-config", "b", "", "备份文件路径")

	configExportCmd.Flags().StringP("config", "c", backupConfigName, fmt.Sprintf("要导出的配置文件名称(%s, %s)", backupConfigName, backupFileMapName))
	configExportCmd.Flags().StringP("output", "o", "", "导出的配置文件")

	configImportCmd.Flags().StringP("config", "c", backupConfigName, fmt.Sprintf("要替换的配置文件名称(%s, %s)", backupConfigName, backupFileMapName))
	configImportCmd.Flags().StringP("import", "i", "", "要导入的配置文件")
	configImportCmd.Flags().BoolP("force", "f", false, "强制替换")

	configCmd.AddCommand(configExportCmd)
	configCmd.AddCommand(configImportCmd)
	rootCmd.AddCommand(configCmd)
}

// exportConfigFromBackup 从备份包中导出配置
func exportConfigFromBackup(zipPath, configName, outputPath string) error {
	configData, err := readFile(zipPath, configName)
	if err != nil {
		return err
	}

	// 写入输出文件
	if err := os.WriteFile(outputPath, configData, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败 (%s): %w", outputPath, err)
	}

	fmt.Printf("配置已成功导出到: %s\n", outputPath)
	return nil
}

// importConfigToBackup 导入配置到备份包
func importConfigToBackup(zipPath, configName, configPath string, force bool) error {
	// 读取配置文件
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败 (%s): %w", configPath, err)
	}

	if !force {
		switch ext := filepath.Ext(configPath); ext {
		case ".yaml", ".yml":
			var cfg map[string]any
			if err := yaml.Unmarshal(configData, &cfg); err != nil {
				return fmt.Errorf("配置文件格式无效: %w", err)
			}
		case ".json":
			var cfg map[string]any
			if err := json.Unmarshal(configData, &cfg); err != nil {
				return fmt.Errorf("配置文件格式无效: %w", err)
			}
		}
	}

	if err := updateZipFile(zipPath, configName, configData); err != nil {
		return fmt.Errorf("更新备份文件失败: %w", err)
	}

	fmt.Printf("配置已成功导入到: %s\n", zipPath)
	return nil
}

// updateZipFile 更新zip文件中的配置文件
func updateZipFile(srcPath, configName string, newConfigData []byte) error {
	// 打开源zip文件
	srcZip, err := zip.OpenReader(srcPath)
	if err != nil {
		return fmt.Errorf("打开源备份文件失败: %w", err)
	}
	defer srcZip.Close()

	srcZip.RegisterDecompressor(zip.Deflate, func(r io.Reader) io.ReadCloser {
		return flate.NewReader(r)
	})

	// 创建目标zip文件
	tempZip, err := os.CreateTemp("", srcPath+".tmp")
	if err != nil {
		return fmt.Errorf("创建目标备份文件失败: %w", err)
	}
	defer os.Remove(tempZip.Name())
	defer tempZip.Close()

	dstZip := zip.NewWriter(tempZip)
	defer dstZip.Close()

	dstZip.RegisterCompressor(zip.Deflate, func(w io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(w, flate.BestCompression)
	})

	// 复制所有文件，替换配置文件
	if !slices.ContainsFunc(srcZip.File, func(f *zip.File) bool { return f.Name == configName }) {
		return fmt.Errorf("备份文件中未找到 %s", configName)
	}

	for _, f := range srcZip.File {
		if f.Name == configName {
			// 写入新的配置文件
			w, err := dstZip.Create(f.Name)
			if err != nil {
				return fmt.Errorf("创建zip条目失败: %w", err)
			}
			if _, err := w.Write(newConfigData); err != nil {
				return fmt.Errorf("写入配置文件失败: %w", err)
			}
		} else {
			// 复制其他文件
			if err := copyZipFile(f, dstZip); err != nil {
				return fmt.Errorf("复制文件失败 (%s): %w", f.Name, err)
			}
		}
	}

	// 替换原文件
	if err := os.Rename(tempZip.Name(), srcPath); err != nil {
		return fmt.Errorf("替换备份文件失败: %w", err)
	}
	return nil
}

// copyZipFile 复制zip文件条目
func copyZipFile(src *zip.File, dst *zip.Writer) error {
	// 打开源文件
	rc, err := src.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	// 创建目标文件
	w, err := dst.Create(src.Name)
	if err != nil {
		return err
	}

	// 复制内容
	_, err = io.Copy(w, rc)
	return err
}

func readFile(zipPath, configName string) ([]byte, error) {
	// 打开备份文件
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("打开备份文件失败 (%s): %w", zipPath, err)
	}
	defer r.Close()

	r.RegisterDecompressor(zip.Deflate, func(r io.Reader) io.ReadCloser {
		return flate.NewReader(r)
	})

	// 查找配置文件
	var configData []byte
	for _, f := range r.File {
		if f.Name == configName {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("打开配置文件失败: %w", err)
			}
			defer rc.Close()

			configData, err = io.ReadAll(rc)
			if err != nil {
				return nil, fmt.Errorf("读取配置文件失败: %w", err)
			}
			break
		}
	}

	if configData == nil {
		return nil, fmt.Errorf("备份文件中未找到 %s", configName)
	}

	return configData, nil
}

var (
	titleStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Right = "├"
		return lipgloss.NewStyle().BorderStyle(b).Padding(0, 1)
	}()

	infoStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Left = "┤"
		return titleStyle.BorderStyle(b)
	}()
)

type model struct {
	title    string
	content  string
	ready    bool
	viewport viewport.Model
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if k := msg.String(); k == "ctrl+c" || k == "q" || k == "esc" {
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		headerHeight := lipgloss.Height(m.headerView())
		footerHeight := lipgloss.Height(m.footerView())
		verticalMarginHeight := headerHeight + footerHeight

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.viewport.YPosition = headerHeight
			m.viewport.SetContent(m.content)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMarginHeight
		}
	}

	// Handle keyboard and mouse events in the viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}
	return fmt.Sprintf("%s\n%s\n%s", m.headerView(), m.viewport.View(), m.footerView())
}

func (m model) headerView() string {
	title := titleStyle.Render(m.title)
	line := strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(title)))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, line)
}

func (m model) footerView() string {
	info := infoStyle.Render(fmt.Sprintf("%3.f%%", m.viewport.ScrollPercent()*100))
	line := strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(info)))
	return lipgloss.JoinHorizontal(lipgloss.Center, line, info)
}
