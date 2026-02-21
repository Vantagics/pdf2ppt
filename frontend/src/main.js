// PDF 转 PPTX 前端逻辑

// 应用状态
let selectedFilePath = '';
let outputFilePath = '';
let isConverting = false;

// DOM 元素引用
const btnSelect = document.getElementById('btn-select');
const btnConvert = document.getElementById('btn-convert');
const fileInfo = document.getElementById('file-info');
const fileName = document.getElementById('file-name');
const filePath = document.getElementById('file-path');
const progressSection = document.getElementById('progress-section');
const progressBar = document.getElementById('progress-bar');
const progressText = document.getElementById('progress-text');
const successSection = document.getElementById('success-section');
const errorSection = document.getElementById('error-section');
const errorMsg = document.getElementById('error-msg');

// ========== 7.2 文件选择功能 ==========

async function selectFile() {
    if (isConverting) return;

    try {
        const path = await window.go.main.App.SelectPDFFile();
        // 用户取消选择时返回空字符串，保持当前状态不变
        if (!path) return;

        selectedFilePath = path;
        // 从路径中提取文件名
        const name = path.split(/[/\\]/).pop();

        fileName.textContent = name;
        filePath.textContent = path;
        fileInfo.style.display = 'block';
        btnConvert.disabled = false;

        // 清除之前的结果
        hideResults();
    } catch (err) {
        showError('选择文件失败: ' + err);
    }
}

// ========== 7.3 转换功能 ==========

async function startConvert() {
    if (isConverting || !selectedFilePath) return;

    try {
        // 获取默认输出路径
        outputFilePath = await window.go.main.App.GetDefaultOutputPath(selectedFilePath);

        // 进入转换状态
        setConverting(true);
        hideResults();
        showProgress();

        // 监听进度事件
        window.runtime.EventsOn('conversion_progress', onProgress);

        // 执行转换
        await window.go.main.App.ConvertPDFToPPTX(selectedFilePath, outputFilePath);

        // 转换成功
        onConvertSuccess();
    } catch (err) {
        // 转换失败
        onConvertError(err);
    } finally {
        // 停止监听进度事件
        window.runtime.EventsOff('conversion_progress');
        setConverting(false);
    }
}

function onProgress(current, total) {
    const pct = total > 0 ? Math.round((current / total) * 100) : 0;
    progressBar.style.width = pct + '%';
    progressText.textContent = '正在转换: ' + current + ' / ' + total + ' 页 (' + pct + '%)';
}

function showProgress() {
    progressSection.style.display = 'block';
    progressBar.style.width = '0%';
    progressText.textContent = '准备中...';
}

// ========== 7.4 转换完成和错误处理 ==========

function onConvertSuccess() {
    progressBar.style.width = '100%';
    progressText.textContent = '转换完成！';
    successSection.style.display = 'block';
    errorSection.style.display = 'none';
}

function onConvertError(err) {
    progressSection.style.display = 'none';
    errorSection.style.display = 'block';
    successSection.style.display = 'none';
    errorMsg.textContent = '转换失败: ' + (err.message || err);
}

async function openOutputDir() {
    if (!outputFilePath) return;
    try {
        await window.go.main.App.OpenOutputDirectory(outputFilePath);
    } catch (err) {
        showError('无法打开目录: ' + err);
    }
}

function showError(msg) {
    errorSection.style.display = 'block';
    successSection.style.display = 'none';
    errorMsg.textContent = msg;
}

function hideResults() {
    successSection.style.display = 'none';
    errorSection.style.display = 'none';
    progressSection.style.display = 'none';
}

// ========== 7.5 UI 状态管理 ==========

function setConverting(converting) {
    isConverting = converting;
    btnSelect.disabled = converting;
    btnConvert.disabled = converting || !selectedFilePath;
}
