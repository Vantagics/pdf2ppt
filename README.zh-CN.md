[English](README.md) | 简体中文

# pdf2ppt

把 PDF 转成可编辑的 PowerPoint 文件（`.pptx`）。每一页 PDF 成为一张幻灯片。文字仍是真正的文本框（不是整页贴上去的图片），图片仍是图片，矢量图形变成形状。

## 给智能体的安装步骤

下面这一段就是全部安装过程。不需要 git，不需要 Go，也不需要先克隆仓库。按机器选一块，整段复制执行，不要改里面的网址。

怎么选：`uname -s` 打印 `Linux` 或 `Darwin` 时用 Linux/macOS 这一段。PowerShell 里 `$env:OS` 等于 `Windows_NT` 时用 Windows 这一段。

### Linux 和 macOS（Cursor）

```bash
#!/bin/sh
set -eu
if ! command -v curl >/dev/null 2>&1 || ! command -v tar >/dev/null 2>&1; then
  if command -v apt-get >/dev/null 2>&1; then
    apt-get update
    apt-get install -y curl ca-certificates tar
  elif command -v apk >/dev/null 2>&1; then
    apk add --no-cache curl ca-certificates tar
  else
    echo "Install curl and tar, then run this script again." >&2
    exit 1
  fi
fi
curl -fsSL https://raw.githubusercontent.com/Vantagics/pdf2ppt/main/skill/pdf2ppt/scripts/install.sh | sh -s -- --skill-dir "$HOME/.cursor/skills"
test -f "$HOME/.cursor/skills/pdf2ppt/SKILL.md"
test -x "$HOME/.local/bin/pdf2ppt"
"$HOME/.local/bin/pdf2ppt" -v
"$HOME/.local/bin/pdf2ppt" "$HOME/.cursor/skills/pdf2ppt/examples/sample.pdf" "$HOME/sample.pptx"
magic=$(od -An -N 2 -t x1 "$HOME/sample.pptx" | tr -d ' \n')
test "$magic" = "504b"
echo PPTX_OK
```

`apt-get` 和 `apk` 需要 root（容器里通常就是 root）。如果 curl 和 tar 已经有了，这一步会跳过。安装程序从最新的 GitHub Release 下载对应系统的二进制，用 `SHA256SUMS` 核对，再写到 `$HOME/.local/bin/pdf2ppt`，同时下载技能文件夹。不必把这个目录加进 `PATH`，直接运行 `"$HOME/.local/bin/pdf2ppt"`。

成功时：脚本会打印 `PDF2PPT_BIN=` 和 `SKILL_MD=`，`pdf2ppt -v` 打出版本号，最后一行是 `PPTX_OK`。

失败时：校验和不对会在安装二进制之前停下来（不用手工删文件，把这一段再跑一遍）。404 表示这个操作系统或 CPU 不在发布文件里，脚本会打印资源名（`pdf2ppt_linux_amd64`、`pdf2ppt_linux_arm64`、`pdf2ppt_darwin_amd64` 或 `pdf2ppt_darwin_arm64`）。`SKILL.md missing` 表示技能包没下下来，再跑一遍这一段。

Claude Code 用同一段，只改一处：`--skill-dir "$HOME/.claude/skills"`，并且 `test -f` 的路径改成 `$HOME/.claude/skills/pdf2ppt/SKILL.md`。Codex 用 `--skill-dir "$HOME/.codex/skills"`。其他读取 SKILL.md 的智能体：把 `--skill-dir` 设成那个应该出现 `pdf2ppt` 文件夹的目录。只给当前项目用时，写成 `--skill-dir "$PWD/.cursor/skills"` 或 `--skill-dir "$PWD/.claude/skills"`。

### Windows（Cursor）

```powershell
$ErrorActionPreference = "Stop"
$dst = Join-Path $env:TEMP "pdf2ppt-install.ps1"
Invoke-WebRequest -Uri "https://raw.githubusercontent.com/Vantagics/pdf2ppt/main/skill/pdf2ppt/scripts/install.ps1" -OutFile $dst
powershell -ExecutionPolicy Bypass -File $dst -SkillDir "$env:USERPROFILE\.cursor\skills"
if (-not (Test-Path "$env:USERPROFILE\.cursor\skills\pdf2ppt\SKILL.md")) { throw "SKILL.md missing" }
$bin = "$env:LOCALAPPDATA\pdf2ppt\pdf2ppt.exe"
& $bin -v
if ($LASTEXITCODE -ne 0) { throw "pdf2ppt -v failed" }
& $bin "$env:USERPROFILE\.cursor\skills\pdf2ppt\examples\sample.pdf" "$env:USERPROFILE\sample.pptx"
if ($LASTEXITCODE -ne 0) { throw "convert failed" }
$bytes = [System.IO.File]::ReadAllBytes("$env:USERPROFILE\sample.pptx")
if ($bytes[0] -ne 0x50 -or $bytes[1] -ne 0x4b) { throw "output is not a pptx" }
Write-Host "PPTX_OK"
```

二进制在 `$env:LOCALAPPDATA\pdf2ppt\pdf2ppt.exe`，不依赖 PATH。Claude Code 把两处 `\.cursor\skills` 改成 `\.claude\skills`。Codex 改成 `\.codex\skills`。

成功时：脚本打印 `PDF2PPT_BIN=`，`-v` 打出版本号，最后一行是 `PPTX_OK`。失败就整段再跑一次。校验和不一致时，不会留下一个半截的二进制文件。

装好之后，新开一个智能体会话，对它说：`convert report.pdf to pptx`。它应该用上面的绝对路径，对 `report.pdf` 运行命令，并写出 `report.pptx`（或你指定的路径）。如果它始终不运行命令，说明技能目录不是这个智能体在读的那个目录。

## 怎么运行

有两种用法：

- **桌面程序**：用 [Wails](https://wails.io) 打开窗口（在仓库根目录执行 `go build` 或 `wails build`）。选一个 PDF，再转换。
- **命令行**：不需要窗口：

```bash
go run ./cmd/pdf2ppt input.pdf output.pptx
```

```text
pdf2ppt 1.3.1
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
