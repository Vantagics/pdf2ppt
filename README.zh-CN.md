[English](README.md) | 简体中文

# pdf2ppt

把 PDF 转成可编辑的 PowerPoint 文件（`.pptx`）。每一页 PDF 成为一张幻灯片。文字仍是真正的文本框（不是整页贴上去的图片），图片仍是图片，矢量图形变成形状。

## 给智能体的安装步骤

Linux 或 macOS（`uname -s` 是 `Linux` 或 `Darwin`）：

```bash
curl -fsSL https://raw.githubusercontent.com/Vantagics/pdf2ppt/main/skill/pdf2ppt/scripts/install.sh | sh
```

Windows PowerShell（`$env:OS` 是 `Windows_NT`）：

```powershell
iex (irm https://raw.githubusercontent.com/Vantagics/pdf2ppt/main/skill/pdf2ppt/scripts/install.ps1)
```

不需要 Go，也不需要 git。这条命令会安装发布版二进制和本技能。技能会写进已经存在的这些目录：`~/.cursor/skills`、`~/.claude/skills`、`~/.codex/skills`。如果一个都不存在，就用 `~/.cursor/skills`。只想指定一个目录时，先设置 `PDF2PPT_SKILL_DIR` 再执行。

检查：`~/.local/bin/pdf2ppt -v` 打出版本号，并且 `~/.cursor/skills/pdf2ppt/SKILL.md` 存在（或你指定的那个目录）。Windows 上是 `& "$env:LOCALAPPDATA\pdf2ppt\pdf2ppt.exe" -v`。然后对智能体说：`convert report.pdf to pptx`。

<details>
<summary>手动安装</summary>

从[最新发布](https://github.com/Vantagics/pdf2ppt/releases/latest)下载 `pdf2ppt_<操作系统>_<架构>`（Windows 带 `.exe`）和 `pdf2ppt-skill.tar.gz`，对照 `SHA256SUMS` 核对。把二进制放到 `~/.local/bin/pdf2ppt`（并 `chmod +x`），或 `%LOCALAPPDATA%\pdf2ppt\pdf2ppt.exe`。解压技能包，使 `SKILL.md` 位于 `~/.cursor/skills/pdf2ppt/SKILL.md`。

</details>

## 怎么运行

有两种用法：

- **桌面程序**：用 [Wails](https://wails.io) 打开窗口（在仓库根目录执行 `go build` 或 `wails build`）。选一个 PDF，再转换。
- **命令行**：不需要窗口：

```bash
go run ./cmd/pdf2ppt input.pdf output.pptx
```

```text
pdf2ppt 1.3.6
usage: pdf2ppt input.pdf output.pptx
```

## 转换会保留什么

程序会遍历 PDF 的内容流（包括压缩过的流），按绘制命令重建幻灯片：

| PDF 里的内容 | PPTX 里的结果 |
| --- | --- |
| 页面尺寸 | 幻灯片尺寸（点换算成 EMU）。宽屏和 4:3 都保持原来的宽高比。 |
| 文本，包括中文 | 可编辑文本框，带字体、字号、粗体、斜体和 RGB 颜色。 |
| 嵌入的图片 | 按原来的位置和大小放 PNG 或 JPEG。软蒙版变成 PNG 的透明通道。 |
| 填充或描边的路径 | 矩形或自由形状，按绘制顺序排列。铺满整页的填充成为幻灯片背景。 |
| 表格、项目符号 | 可编辑的文字，加上 PDF 里实际画出的矩形和符号，不是 PowerPoint 的原生表格。 |

字体名来自 PDF 的 `BaseFont`（会去掉 `ABCDEF+` 这种子集前缀），再映射成 LibreOffice 能识别的家族名，例如 `LiberationSans-Bold` 变成 Liberation Sans 加粗，`WenQuanYiMicroHei` 变成 WenQuanYi Micro Hei。字符编码走字体的 ToUnicode 表，所以中文是 Unicode，不是原始字形编号。

## 往返检查

实际用来看保真度的做法是：

1. 从一份 PPTX 开始。
2. 把它导出成 PDF（测试用的是 LibreOffice Impress 的无界面模式）。
3. 用 pdf2ppt 把这份 PDF 转回去。
4. 比较两份 PPTX。

`tests/roundtrip` 里的样例在往返之后，幻灯片数量一致，宽高比误差在 1.5 点以内，文字一致（忽略空白和项目符号）。把两份文稿重新渲染后做 SSIM，每一页都在 **0.99 以上**。前后对比的数字写在更新日志里。

测试是 `converter/roundtrip_test.go` 里的 `TestRoundTrip`。需要：

- Go 1.22 或更高版本
- LibreOffice Impress（`soffice`）
- Poppler（`pdftoppm`）
- Python 3，以及 `python-pptx`、`pillow`、`numpy`、`scikit-image`

```bash
go test ./converter -run 'TestRoundTrip|TestRejectsNonPDF' -count=1 -timeout 20m
```

`tests/roundtrip/generate_samples.py` 生成样例（文字样式、中日韩、项目符号、图片、形状、表格、纯色和图片背景、4:3 幻灯片，以及混合页）。`tests/roundtrip/compare.py` 比较一对结果并打印 JSON。Go 测试把这些和转换器串在一起。

不依赖 LibreOffice 的解析测试：

```bash
go test ./converter/gopdf2 ./converter -count=1 -run 'TestNormalize|TestToUnicode|TestTokenizer|TestRejects|TestEmpty'
```

## 真实幻灯片

`tests/corpus/manifest.json` 列出核对过制作程序（Producer）的演示文稿 PDF，包括 PowerPoint 2016、Microsoft 365 的 PowerPoint、Keynote，以及用 Chrome 把 `tests/corpus/slides.html` 打印出来的 Skia/PDF（Google 幻灯片用的是同一套 PDF 后端）。这些下载的文件没有放进仓库：要么许可证不明确，要么是别人的演讲稿。`tests/corpus/fetch.py` 会把它们下载到 `tests/corpus/cache/`。

```bash
python3 tests/corpus/fetch.py
go test ./converter -run 'TestRoundTrip|TestChromeSkiaSlide' -count=1 -timeout 20m
```

在装了 PowerPoint 的 Windows 上：

```powershell
powershell -STA -File tools/ppt-verify.ps1 -Path deck.pptx -Slides 1,23 -OutDir ppt-verify
```

这条命令用 PowerPoint 打开文件，打印幻灯片数量和尺寸，并按要求把每一页导出成 PNG。需要修复，或对话框一直不消失，都算失败。

已经核对过的 PowerPoint 和 Keynote 页面能保住图片和版式。加泰罗尼亚语里的 à è é í ó 需要 WinAnsi，因为 PowerPoint 在没有 ToUnicode 时用的就是这种编码。Chrome/Skia 的渐变是轴向着色；旋转的文字是真正旋转的文本框。PowerPoint 2013 经常把幻灯片背景存成只有一张图的平铺图案，这张照片会铺到幻灯片上，而不是变成黑底。Wingdings 的专用区箭头会变成 Unicode 箭头。每份文稿的 SSIM 表在 `tests/corpus/COMPATIBILITY.md`。没有找到可以再分发的 WPS 演示文稿导出文件。

## 已知限制

- **一份文件只有一个幻灯片尺寸。** OOXML 只存一个 `sldSz`。这个尺寸取出现次数最多的页面；次数相同则用第一页。尺寸不同的页按原比例缩小后居中放到这张幻灯片上，不拉伸，较小的页也不放大。多出来的边是留白（letterbox）。
- **一行文字一个文本框。** 同一基线上、彼此挨着的几段文字合成一个文本框；样式变了就在框里另起一段文字。自动换行的段落仍然是每行一个框。间隔很大的两段（例如两栏）仍然是两个框。
- **表格和项目符号不是 PowerPoint 的原生对象。** 表格是单元格矩形加上文本框。项目符号是符号字符加上每行一个文本框。两者都可以继续编辑。
- **预设形状是轮廓。** 矩形仍然是矩形。椭圆、圆角矩形、菱形和其他曲线会变成与 PDF 路径一致的自由形状。LibreOffice 栅格化的投影会保留成图片。
- **基线按水平文字来放。** 文本框的位置让 LibreOffice 把基线画在 PDF 的基线上。旋转过的文字是旋转的文本框。竖排书写模式不会重建。
- **字体要装在本机上才会长得一样。** 文件里记录的是字体名，没有嵌入字体文件。缺了这个家族，查看器会换一个字体。
- **不是每种 PDF 结构都有对应处理。** 轴向和径向着色会变成渐变填充；只有一张图的平铺图案（PowerPoint 背景常见这种写法）会按图片放上去。其他平铺图案、Type 3 字体、可选内容、注释，以及需要用户口令的加密文件，仍会被跳过或拒绝。内联图片不会读取。

## 目录

```text
cmd/pdf2ppt/          命令行
converter/            转换流程
converter/gopdf2/     PDF 内容解释器
converter/goppt/      PPTX 写入
frontend/             Wails 界面
skill/pdf2ppt/        智能体技能（SKILL.md 和安装脚本）
tests/roundtrip/      样例生成和 SSIM 比较
```
