package converter

import (
	"fmt"
	"os"
)

// ProgressCallback 进度回调函数类型
type ProgressCallback func(current, total int)

// Converter PDF 转可编辑 PPTX 转换引擎
type Converter struct {
	onProgress ProgressCallback
}

// NewConverter 创建转换引擎实例
func NewConverter(onProgress ProgressCallback) *Converter {
	return &Converter{onProgress: onProgress}
}

// Convert 执行转换：提取PDF元素 → 生成可编辑PPTX
// 转换失败时清理临时文件和不完整的输出文件
func (c *Converter) Convert(inputPath, outputPath string) (result *ConvertResult, err error) {
	// Task 4.5: panic 恢复和失败时输出文件清理
	defer func() {
		if r := recover(); r != nil {
			err = &ConvertError{
				Code:    ErrCodeInternalError,
				Message: fmt.Sprintf("转换过程中发生内部错误: %v", r),
			}
			os.Remove(outputPath)
		}
	}()

	// Task 4.2: 打开 PDF
	parser := NewPDFParser()
	if err := parser.Open(inputPath); err != nil {
		return nil, err
	}
	defer parser.Close()

	// Task 4.4: 空 PDF 检查
	totalPages := parser.GetPageCount()
	if totalPages == 0 {
		return nil, &ConvertError{
			Code:    ErrCodeEmptyPDF,
			Message: "PDF 文件不包含任何页面",
		}
	}

	// Task 4.2: 创建 PPTX 生成器
	generator := NewPPTXGenerator()
	defer generator.Close()

	// Task 4.2: 循环提取页面内容并创建幻灯片
	for i := 0; i < totalPages; i++ {
		content, err := parser.ExtractPageContent(i)
		if err != nil {
			os.Remove(outputPath)
			return nil, err
		}

		if err := generator.AddSlideFromContent(content); err != nil {
			os.Remove(outputPath)
			return nil, err
		}

		// Task 4.3: 进度回调
		if c.onProgress != nil {
			c.onProgress(i+1, totalPages)
		}
	}

	// Task 4.2: 保存 PPTX
	if err := generator.Save(outputPath); err != nil {
		os.Remove(outputPath)
		return nil, err
	}

	return &ConvertResult{
		OutputPath: outputPath,
		PageCount:  totalPages,
	}, nil
}
