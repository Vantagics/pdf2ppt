[English](README.md) | 简体中文

# pdf2ppt

把 PDF 转成可编辑的 PowerPoint 文件（`.pptx`）。每一页 PDF 成为一张幻灯片。文字仍是真正的文本框（不是整页贴上去的图片），图片仍是图片，矢量图形变成形状。

## 智能体技能

`skill/pdf2ppt` 是一份智能体技能。Cursor、Claude Code 或 Codex 加载它之后，你只要说「把这个 PDF 转成 PPT」，它就会安装 `pdf2ppt` 命令并完成转换。智能体实际遵循的说明写在 `skill/pdf2ppt/SKILL.md`。

把这个文件夹复制到你的智能体读取技能的目录里即可。

Cursor，Linux 或 macOS：

```bash
mkdir -p ~/.cursor/skills
cp -R skill/pdf2ppt ~/.cursor/skills/pdf2ppt
```

只在当前项目里给 Cursor 用：

```bash
mkdir -p .cursor/skills
cp -R skill/pdf2ppt .cursor/skills/pdf2ppt
```

Cursor，Windows PowerShell：

```powershell
New-Item -ItemType Directory -Force -Path "$env:USERPROFILE\.cursor\skills" | Out-Null
Copy-Item -Recurse -Force skill\pdf2ppt "$env:USERPROFILE\.cursor\skills\pdf2ppt"
```

Claude Code，Linux 或 macOS（用户目录，或项目里的 `.claude/skills/`）：

```bash
mkdir -p ~/.claude/skills
cp -R skill/pdf2ppt ~/.claude/skills/pdf2ppt
```

Claude Code，Windows PowerShell：

```powershell
New-Item -ItemType Directory -Force -Path "$env:USERPROFILE\.claude\skills" | Out-Null
Copy-Item -Recurse -Force skill\pdf2ppt "$env:USERPROFILE\.claude\skills\pdf2ppt"
```

Codex 以及其他读取 `SKILL.md` 的智能体也是同样的做法：把 `skill/pdf2ppt` 复制到它们自己的 skills 目录，最终路径以 `.../skills/pdf2ppt/SKILL.md` 结尾。如果手头只有仓库的压缩包，从 GitHub 下载并解压，再从里面复制 `skill/pdf2ppt` 文件夹。

技能要调用 `pdf2ppt` 这个命令。在克隆下来的仓库里：

```bash
bash skill/pdf2ppt/scripts/install.sh
pdf2ppt -v
```

Windows PowerShell：

```powershell
powershell -ExecutionPolicy Bypass -File skill\pdf2ppt\scripts\install.ps1
pdf2ppt -v
```

脚本会先保留已经装好的命令；没有的话就编译当前仓库；再不行就执行 `go install github.com/Vantagics/pdf2ppt/cmd/pdf2ppt@latest`；还不行就从最新的 GitHub Release 下载 `pdf2ppt_<操作系统>_<架构>`。`go install` 会把程序写到 `$(go env GOPATH)/bin`。如果接着运行 `pdf2ppt -v` 提示找不到命令，把这个目录加进 `PATH`。

新开一个智能体会话，让它重新加载技能。可以这样说：`convert report.pdf to pptx`。它应该执行 `pdf2ppt report.pdf report.pptx`，并告诉你文件写到了哪里。如果它只是口头回答、没有去运行命令，说明技能目录不在这个智能体读取的路径上。

## 怎么运行

有两种用法：

- **桌面程序**：用 [Wails](https://wails.io) 打开窗口（在仓库根目录执行 `go build` 或 `wails build`）。选一个 PDF，再转换。
- **命令行**：不需要窗口：

```bash
go run ./cmd/pdf2ppt input.pdf output.pptx
```

```text
pdf2ppt 1.3.0
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

已经核对过的 PowerPoint 和 Keynote 页面能保住图片和版式。加泰罗尼亚语里的 à è é í ó 需要 WinAnsi，因为 PowerPoint 在没有 ToUnicode 时用的就是这种编码。Chrome/Skia 的渐变是轴向着色；旋转的文字是真正旋转的文本框。PowerPoint 2013 经常把幻灯片背景存成只有一张图的平铺图案，这张照片会铺到幻灯片上，而不是变成黑底。Wingdings 的专用区箭头会变成 Unicode 箭头。每份文稿的 SSIM 表在 `tests/corpus/COMPATIBILITY.md`。没有找到可以再分发的 WPS 演示文稿导出文件。

## 已知限制

- **一份文件只有一个幻灯片尺寸。** OOXML 只存一个 `sldSz`。演示文稿用最大的那一页做画布，后面更大的页不会被裁掉。较小的页仍用自己的框，留在这块画布的一角。
- **一段 PDF 文字对应一个文本框。** 自动换行的段落，或同一行里中英混排，通常会变成同一基线上的好几个框，而不是原来的那一个文本框。字符和位置还在。
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
