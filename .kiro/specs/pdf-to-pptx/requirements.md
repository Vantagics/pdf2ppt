# 需求文档

## 简介

本功能为基于 Wails 框架的 Go 桌面应用提供 PDF 转可编辑 PPTX 的能力。用户可以选择一个 PDF 文件，应用将使用 GoPDF2 库解析 PDF 中的文本内容、图片元素及其格式与布局信息，并使用 GoPPT 库生成对应的可编辑 PPTX 文件。每一页 PDF 将转换为 PPTX 中的一张幻灯片，其中文本以可编辑文本框形式呈现，图片以独立图片元素插入，尽可能保留原始的字体、大小、颜色、位置等格式信息和页面布局。

## 术语表

- **Converter**: PDF 转 PPTX 的核心转换引擎，负责协调解析和生成流程
- **PDF_Parser**: 基于 GoPDF2 库的 PDF 解析模块，负责读取和提取 PDF 页面中的文本、图片及布局信息
- **PPTX_Generator**: 基于 GoPPT 库的 PPTX 生成模块，负责创建包含可编辑元素的幻灯片并写入文件
- **Wails_Frontend**: Wails 框架的前端界面，负责用户交互
- **Wails_Backend**: Wails 框架的后端 Go 服务，负责暴露转换 API 给前端调用
- **Text_Element**: 从 PDF 页面中提取的文本元素，包含文本内容、字体名称、字体大小、字体颜色、位置坐标（X、Y）和尺寸（宽度、高度）
- **Image_Element**: 从 PDF 页面中提取的图片元素，包含图片二进制数据、图片格式、位置坐标（X、Y）和尺寸（宽度、高度）
- **Page_Content**: 一个 PDF 页面中提取的所有元素集合，包含该页的 Text_Element 列表、Image_Element 列表以及页面宽高信息
- **Slide**: PPTX 文件中的一张幻灯片
- **Conversion_Progress**: 转换过程中的进度信息，包含当前页码和总页数

## 需求

### 需求 1：PDF 文件选择

**用户故事：** 作为用户，我想通过桌面应用选择一个 PDF 文件，以便将其转换为可编辑的 PPTX 格式。

#### 验收标准

1. WHEN 用户点击"选择文件"按钮, THE Wails_Frontend SHALL 打开系统原生文件选择对话框，且文件类型过滤器仅显示 PDF 文件（*.pdf）
2. WHEN 用户选择了一个 PDF 文件, THE Wails_Frontend SHALL 显示所选文件的文件名和文件路径
3. WHEN 用户取消文件选择对话框, THE Wails_Frontend SHALL 保持当前界面状态不变，不显示错误信息

### 需求 2：PDF 文件验证

**用户故事：** 作为用户，我想在转换前验证所选 PDF 文件的有效性，以便避免无效文件导致转换失败。

#### 验收标准

1. WHEN 用户选择了一个 PDF 文件, THE PDF_Parser SHALL 验证该文件是否为有效的 PDF 格式
2. IF 所选文件不是有效的 PDF 格式, THEN THE Converter SHALL 返回包含"文件不是有效的 PDF 格式"的错误信息
3. IF 所选 PDF 文件的页数为零, THEN THE Converter SHALL 返回包含"PDF 文件不包含任何页面"的错误信息
4. IF 所选文件路径不存在或无法读取, THEN THE Converter SHALL 返回包含"无法读取文件"的错误信息

### 需求 3：PDF 页面内容提取

**用户故事：** 作为用户，我想让应用从 PDF 的每一页中提取文本和图片等可编辑元素，以便转换后的 PPTX 中的内容可以直接编辑。

#### 验收标准

1. WHEN 一个有效的 PDF 文件被提交转换, THE PDF_Parser SHALL 提取 PDF 文件的总页数
2. WHEN PDF_Parser 解析每一页时, THE PDF_Parser SHALL 提取该页的宽度和高度尺寸信息（单位为磅/points）
3. WHEN PDF_Parser 解析每一页时, THE PDF_Parser SHALL 提取该页中所有文本内容，并为每段文本生成一个 Text_Element，包含文本内容、字体名称、字体大小（单位为磅）、字体颜色（RGB 值）、位置坐标（X、Y，单位为磅）和文本区域尺寸（宽度、高度，单位为磅）
4. WHEN PDF_Parser 解析每一页时, THE PDF_Parser SHALL 提取该页中所有嵌入图片，并为每张图片生成一个 Image_Element，包含图片二进制数据、图片格式（如 PNG、JPEG）、位置坐标（X、Y，单位为磅）和图片尺寸（宽度、高度，单位为磅）
5. WHEN PDF_Parser 完成一页的解析后, THE PDF_Parser SHALL 将该页的所有 Text_Element 和 Image_Element 组合为一个 Page_Content 对象
6. IF PDF_Parser 在提取某一页的文本或图片时发生错误, THEN THE Converter SHALL 返回包含页码信息的错误描述
7. IF 某一页中的文本字体信息无法完整提取, THEN THE PDF_Parser SHALL 使用默认字体（Arial）和默认字体大小（12磅）作为该 Text_Element 的字体属性

### 需求 4：可编辑 PPTX 文件生成

**用户故事：** 作为用户，我想将 PDF 的每一页转换为 PPTX 中包含可编辑文本框和独立图片的幻灯片，以便在 PowerPoint 中直接编辑文本和调整图片。

#### 验收标准

1. THE PPTX_Generator SHALL 为 PDF 的每一页创建一张对应的 Slide
2. WHEN 创建每张 Slide 时, THE PPTX_Generator SHALL 根据对应 PDF 页面的宽高尺寸设置 Slide 的尺寸（将磅值转换为 EMU 单位）
3. WHEN 处理 Page_Content 中的每个 Text_Element 时, THE PPTX_Generator SHALL 在 Slide 上对应位置创建一个可编辑文本框，文本框的位置和尺寸与 Text_Element 中记录的坐标和尺寸一致
4. WHEN 创建文本框时, THE PPTX_Generator SHALL 设置文本的字体名称、字体大小和字体颜色，使其与 Text_Element 中记录的格式信息一致
5. WHEN 处理 Page_Content 中的每个 Image_Element 时, THE PPTX_Generator SHALL 在 Slide 上对应位置插入一张独立图片，图片的位置和尺寸与 Image_Element 中记录的坐标和尺寸一致
6. THE PPTX_Generator SHALL 按照 PDF 页面的原始顺序排列生成的 Slide
7. WHEN 所有 Slide 创建完成后, THE PPTX_Generator SHALL 将 PPTX 文件写入用户指定的输出路径
8. IF PPTX_Generator 在创建某个文本框或插入某张图片时发生错误, THEN THE PPTX_Generator SHALL 跳过该元素并继续处理剩余元素，同时记录警告日志

### 需求 5：输出路径选择

**用户故事：** 作为用户，我想指定 PPTX 文件的保存位置，以便将转换结果保存到我想要的目录。

#### 验收标准

1. WHEN 用户点击"转换"按钮, THE Wails_Frontend SHALL 打开系统原生的"另存为"对话框，默认文件名为原 PDF 文件名（扩展名替换为 .pptx）
2. WHEN 用户指定了保存路径, THE Wails_Backend SHALL 使用该路径作为 PPTX 文件的输出路径
3. WHEN 用户取消"另存为"对话框, THE Wails_Frontend SHALL 取消本次转换操作，不执行任何转换逻辑
4. IF 指定的输出路径无法写入, THEN THE Converter SHALL 返回包含"无法写入目标路径"的错误信息

### 需求 6：转换进度反馈

**用户故事：** 作为用户，我想在转换过程中看到进度信息，以便了解转换的当前状态。

#### 验收标准

1. WHILE 转换正在进行中, THE Wails_Backend SHALL 向 Wails_Frontend 发送 Conversion_Progress 事件，包含当前处理的页码和总页数
2. WHILE 转换正在进行中, THE Wails_Frontend SHALL 显示进度条，进度百分比为（当前页码 / 总页数）× 100
3. WHEN 转换成功完成, THE Wails_Frontend SHALL 显示"转换完成"的提示信息，并提供"打开文件所在目录"的按钮
4. WHILE 转换正在进行中, THE Wails_Frontend SHALL 禁用"选择文件"和"转换"按钮，防止重复操作

### 需求 7：错误处理与用户提示

**用户故事：** 作为用户，我想在转换失败时看到清晰的错误提示，以便了解问题原因。

#### 验收标准

1. IF 转换过程中发生任何错误, THEN THE Wails_Frontend SHALL 在界面上显示错误信息对话框，包含具体的错误描述
2. IF 转换过程中发生错误, THEN THE Converter SHALL 清理已生成的临时文件和不完整的输出文件
3. WHEN 错误信息对话框显示后, THE Wails_Frontend SHALL 允许用户重新选择文件并再次尝试转换
4. IF GoPDF2 库或 GoPPT 库抛出未预期的异常, THEN THE Converter SHALL 捕获该异常并返回包含"转换过程中发生内部错误"的错误信息

### 需求 8：Wails 后端 API

**用户故事：** 作为开发者，我想通过 Wails 绑定的 API 调用转换功能，以便前端可以触发和控制转换流程。

#### 验收标准

1. THE Wails_Backend SHALL 暴露一个 `SelectPDFFile` 方法，调用后打开文件选择对话框并返回所选文件路径
2. THE Wails_Backend SHALL 暴露一个 `ConvertPDFToPPTX` 方法，接收 PDF 文件路径和 PPTX 输出路径作为参数，执行转换并返回转换结果
3. THE Wails_Backend SHALL 暴露一个 `OpenOutputDirectory` 方法，接收文件路径参数，调用系统命令打开该文件所在的目录
4. WHEN `ConvertPDFToPPTX` 方法被调用时, THE Wails_Backend SHALL 通过 Wails 事件机制向前端发送 Conversion_Progress 事件
