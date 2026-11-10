package main

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"github.com/Vantagics/pdf2ppt/converter"
	"runtime"
	"strings"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App 是 Wails 应用的主结构体
type App struct {
	ctx       context.Context
	converter *converter.Converter
}

// NewApp 创建 App 实例
func NewApp() *App {
	return &App{}
}

// startup 在 Wails 应用启动时调用
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.converter = converter.NewConverter(func(current, total int) {
		wailsRuntime.EventsEmit(a.ctx, "conversion_progress", current, total)
	})
}

// shutdown 在 Wails 应用关闭时调用
func (a *App) shutdown(ctx context.Context) {
}

// SelectPDFFile 打开文件选择对话框，返回所选 PDF 文件路径
// 返回空字符串表示用户取消了选择
func (a *App) SelectPDFFile() (string, error) {
	selection, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "选择 PDF 文件",
		Filters: []wailsRuntime.FileFilter{
			{
				DisplayName: "PDF 文件 (*.pdf)",
				Pattern:     "*.pdf",
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("打开文件选择对话框失败: %w", err)
	}
	return selection, nil
}

// ConvertPDFToPPTX 执行 PDF 到可编辑 PPTX 的转换
// inputPath: PDF 文件路径
// outputPath: PPTX 输出路径
func (a *App) ConvertPDFToPPTX(inputPath, outputPath string) error {
	if inputPath == "" || outputPath == "" {
		return fmt.Errorf("输入路径和输出路径不能为空")
	}
	_, err := a.converter.Convert(inputPath, outputPath)
	return err
}

// OpenOutputDirectory 打开文件所在目录
func (a *App) OpenOutputDirectory(filePath string) error {
	if filePath == "" {
		return fmt.Errorf("文件路径不能为空")
	}
	dir := filepath.Dir(filePath)

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", dir)
	case "darwin":
		cmd = exec.Command("open", dir)
	default: // linux and others
		cmd = exec.Command("xdg-open", dir)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("无法打开目录: %w", err)
	}
	return nil
}

// GetDefaultOutputPath 根据输入 PDF 路径生成默认的 PPTX 输出路径
func (a *App) GetDefaultOutputPath(inputPath string) string {
	ext := filepath.Ext(inputPath)
	if strings.EqualFold(ext, ".pdf") {
		return inputPath[:len(inputPath)-len(ext)] + ".pptx"
	}
	return inputPath + ".pptx"
}
