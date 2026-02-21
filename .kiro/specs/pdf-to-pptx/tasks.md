# 任务列表：PDF 转可编辑 PPTX

## 任务 1：项目初始化与数据模型

- [x] 1.1 初始化 Wails v2 项目结构（main.go, app.go, wails.json, frontend/）
- [x] 1.2 创建 go.mod，添加 GoPDF2 和 GoPPT 依赖
- [x] 1.3 创建 `converter/models.go`，定义 TextElement、ImageElement、PageContent、ConversionProgress、ConvertResult、ConvertError 结构体和默认值常量
- [x] 1.4 创建 `converter/models.go` 中的 PointsToEMU 单位转换函数
- [x] 1.5 创建 `converter/models_test.go`，编写 PointsToEMU 单位转换的单元测试

## 任务 2：PDFParser 实现

- [x] 2.1 创建 `converter/pdf_parser.go`，实现 PDFParser 结构体和 NewPDFParser 构造函数
- [x] 2.2 实现 `Open` 方法：打开并验证 PDF 文件（文件存在性、可读性、有效 PDF 格式）
- [x] 2.3 实现 `GetPageCount` 方法：获取 PDF 总页数
- [x] 2.4 实现 `ExtractPageContent` 方法：提取指定页面的宽高尺寸
- [x] 2.5 实现 `ExtractPageContent` 方法中的文本元素提取逻辑：遍历页面文本对象，生成 TextElement（含内容、字体、大小、颜色、位置、尺寸），字体缺失时使用默认值
- [x] 2.6 实现 `ExtractPageContent` 方法中的图片元素提取逻辑：遍历页面图片对象，生成 ImageElement（含二进制数据、格式、位置、尺寸）
- [x] 2.7 实现 `Close` 方法：关闭 PDF 文档释放资源
- [x] 2.8 创建 `converter/pdf_parser_test.go`，使用 test.pdf 编写 PDFParser 的单元测试（Open、GetPageCount、ExtractPageContent）

## 任务 3：PPTXGenerator 实现

- [x] 3.1 创建 `converter/pptx_generator.go`，实现 PPTXGenerator 结构体和 NewPPTXGenerator 构造函数
- [x] 3.2 实现 `AddSlideFromContent` 方法：根据 PageContent 创建幻灯片并设置尺寸
- [x] 3.3 实现 `AddSlideFromContent` 中的文本框创建逻辑：为每个 TextElement 创建可编辑文本框，设置位置、尺寸、字体名称、大小、颜色
- [x] 3.4 实现 `AddSlideFromContent` 中的图片插入逻辑：为每个 ImageElement 插入独立图片，设置位置和尺寸
- [x] 3.5 实现 `AddSlideFromContent` 中的元素级错误容忍：单个元素失败时跳过并记录警告
- [x] 3.6 实现 `Save` 方法：将 PPTX 文件保存到指定路径
- [x] 3.7 实现 `Close` 方法：关闭并清理资源
- [x] 3.8 创建 `converter/pptx_generator_test.go`，编写 PPTXGenerator 的单元测试（AddSlideFromContent、Save）

## 任务 4：Converter 转换引擎实现

- [x] 4.1 创建 `converter/converter.go`，实现 Converter 结构体、NewConverter 构造函数和 ProgressCallback 类型
- [x] 4.2 实现 `Convert` 方法核心流程：打开 PDF → 循环提取页面内容 → 创建可编辑幻灯片 → 保存 PPTX
- [x] 4.3 实现 `Convert` 方法中的进度回调：每处理完一页发送进度事件
- [x] 4.4 实现 `Convert` 方法中的错误处理：文件验证错误、页面提取错误、写入错误
- [x] 4.5 实现 `Convert` 方法中的 panic 恢复和失败时输出文件清理
- [x] 4.6 创建 `converter/converter_test.go`，编写 Converter 的单元测试（使用 test.pdf 端到端测试、错误场景测试）

## 任务 5：属性测试

- [x] 5.1 在 `converter/converter_test.go` 中添加属性测试：属性 1（页面数量守恒）— 生成随机 PageContent 列表，验证幻灯片数量等于页面数量
- [x] 5.2 在 `converter/pptx_generator_test.go` 中添加属性测试：属性 2（页面尺寸保持）— 生成随机页面尺寸，验证磅到 EMU 转换正确性
- [x] 5.3 在 `converter/pptx_generator_test.go` 中添加属性测试：属性 3（文本元素保真）— 生成随机 TextElement，验证文本框属性一致
- [x] 5.4 在 `converter/pptx_generator_test.go` 中添加属性测试：属性 4（图片元素保真）— 生成随机 ImageElement，验证图片属性一致
- [x] 5.5 在 `converter/pdf_parser_test.go` 中添加属性测试：属性 5（字体缺失默认值）— 生成缺失字体信息的文本元素，验证默认值回退
- [x] 5.6 在 `converter/pptx_generator_test.go` 中添加属性测试：属性 6（幻灯片顺序保持）— 生成随机多页内容，验证顺序一致
- [x] 5.7 在 `converter/pptx_generator_test.go` 中添加属性测试：属性 7（元素级错误容忍）— 注入失败元素，验证其余元素正常处理
- [x] 5.8 在 `converter/converter_test.go` 中添加属性测试：属性 8（进度事件完整性）— 生成随机页数，验证回调次数和参数
- [x] 5.9 在 `converter/converter_test.go` 中添加属性测试：属性 9（失败时文件清理）— 模拟转换失败，验证输出文件被清理
- [x] 5.10 在 `converter/pdf_parser_test.go` 中添加属性测试：属性 10（PDF 格式验证）— 生成随机字节数据，验证被拒绝

## 任务 6：Wails 后端 API

- [x] 6.1 在 `app.go` 中实现 App 结构体，包含 context 和 Converter 实例
- [x] 6.2 实现 `SelectPDFFile` 方法：调用 Wails runtime 打开文件选择对话框，过滤 PDF 文件
- [x] 6.3 实现 `ConvertPDFToPPTX` 方法：调用 Converter.Convert，通过 Wails 事件发送进度
- [x] 6.4 实现 `OpenOutputDirectory` 方法：调用系统命令打开文件所在目录
- [x] 6.5 实现 `GetDefaultOutputPath` 方法：将 .pdf 扩展名替换为 .pptx

## 任务 7：前端界面

- [x] 7.1 创建 `frontend/index.html` 和 `frontend/src/main.js`，实现基础 UI 布局（文件选择按钮、文件信息显示、转换按钮、进度条）
- [x] 7.2 实现文件选择功能：调用 SelectPDFFile，显示文件名和路径
- [x] 7.3 实现转换功能：调用 ConvertPDFToPPTX，监听进度事件更新进度条
- [x] 7.4 实现转换完成和错误处理 UI：成功提示、打开目录按钮、错误对话框
- [x] 7.5 实现 UI 状态管理：转换中禁用按钮，完成/失败后恢复
