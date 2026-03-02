<template>
    <div class="grain"></div>
    <main class="shell">
        <header class="hero">
            <div>
                <p class="kicker">本地媒体检测</p>
                <h1>minfo</h1>
                <p class="lead">一键生成 MediaInfo / BDInfo，并下载 8 张截图。</p>
            </div>
        </header>

        <section class="panel">
            <div class="field">
                <label for="selected-path">媒体路径</label>
                <div class="path-picker">
                    <div class="path-selected">
                        <span class="path-icon">📁</span>
                        <input
                            id="selected-path"
                            type="text"
                            :value="path"
                            placeholder="请选择文件或文件夹"
                            readonly
                        />
                    </div>
                    <div class="path-actions">
                        <button class="ghost" :disabled="busy" @click="openPicker">选择文件或文件夹</button>
                        <button class="ghost" :disabled="busy || path.trim() === ''" @click="clearPath">清空路径</button>
                    </div>
                    <div class="path-hint">选择明确的媒体文件时，会直接按该文件进行分析。</div>
                    <div class="browser" v-if="pickerOpen">
                        <div class="browser-toolbar">
                            <div class="browser-current">{{ browserDir || browserRoot || "加载中..." }}</div>
                            <div class="browser-buttons">
                                <button class="ghost" :disabled="busy || browserLoading || isAtRoot" @click="navigateUp">上一级</button>
                                <button class="ghost" :disabled="busy || browserLoading" @click="refreshBrowser">刷新</button>
                                <button class="ghost" :disabled="busy" @click="closePicker">关闭</button>
                            </div>
                        </div>
                        <div class="browser-error" v-if="browserError !== ''">
                            {{ browserError }}
                        </div>
                        <div class="browser-list">
                            <div class="browser-row">
                                <span class="browser-row-name">当前目录</span>
                                <div class="browser-row-actions">
                                    <button class="ghost" :disabled="busy || browserLoading || !browserDir" @click="chooseCurrentDir">
                                        选择此文件夹
                                    </button>
                                </div>
                            </div>
                            <div class="browser-row empty" v-if="!browserLoading && browserEntries.length === 0">
                                目录为空
                            </div>
                            <div class="browser-row empty" v-if="browserLoading">
                                加载中...
                            </div>
                            <div class="browser-row" v-for="entry in browserEntries" :key="entry.path">
                                <span class="browser-row-name">{{ entry.name }}</span>
                                <div class="browser-row-actions">
                                    <button class="ghost" v-if="entry.isDir" :disabled="busy || browserLoading" @click="enterDir(entry.path)">
                                        进入
                                    </button>
                                    <button class="ghost" :disabled="busy || browserLoading" @click="choosePath(entry.path)">
                                        {{ entry.isDir ? "选择文件夹" : "选择文件" }}
                                    </button>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
            <div class="actions">
                <button :disabled="busy" @click="runInfo('/api/mediainfo', 'MediaInfo')">生成 MediaInfo</button>
                <button :disabled="busy" @click="runInfo('/api/bdinfo', 'BDInfo')">生成 BDInfo</button>
                <button :disabled="busy" @click="downloadShots">下载 8 张截图</button>
            </div>
        </section>

        <section class="panel output">
            <div class="output-header">
                <h2>输出</h2>
                <div class="output-actions">
                    <button class="ghost" @click="copyOutput">{{ copyLabel }}</button>
                    <button class="ghost" :disabled="busy" @click="clearOutput">清空</button>
                </div>
            </div>
            <pre>{{ output }}</pre>
        </section>

        <footer class="footer">
            <p>从服务器媒体目录中选择文件或文件夹后再执行分析。</p>
        </footer>
    </main>
</template>

<script setup>
import { computed, onBeforeUnmount, ref } from "vue";

const path = ref("");
const output = ref("就绪。");
const busy = ref(false);
const copyLabel = ref("复制");

const pickerOpen = ref(false);
const browserRoot = ref("");
const browserDir = ref("");
const browserEntries = ref([]);
const browserLoading = ref(false);
const browserError = ref("");
let browserController = null;

const hasInput = () => path.value.trim() !== "";

const normalizeComparePath = (value) => {
    if (!value) {
        return "";
    }
    if (value === "/" || value === "\\") {
        return "/";
    }
    return value.replace(/\\/g, "/").replace(/\/+$/, "");
};

const isAtRoot = computed(() => {
    const root = normalizeComparePath(browserRoot.value);
    const current = normalizeComparePath(browserDir.value);
    if (root === "" || current === "") {
        return false;
    }
    return root === current;
});

const clearPath = () => {
    path.value = "";
};

const withTrailingSeparator = (value) => {
    if (value === "") {
        return "";
    }
    if (value.endsWith("/") || value.endsWith("\\")) {
        return value;
    }
    const separator = value.includes("\\") && !value.includes("/") ? "\\" : "/";
    return `${value}${separator}`;
};

const cleanPath = (value) => {
    if (!value) {
        return "";
    }
    if (value === "/" || value === "\\") {
        return value;
    }
    return value.replace(/[\\/]+$/, "");
};

const getEntryName = (value) => {
    const normalized = value.replace(/[\\/]+$/, "");
    if (normalized === "") {
        return value;
    }
    const parts = normalized.split(/[\\/]/);
    return parts[parts.length - 1] || normalized;
};

const buildEntries = (items) => {
    const result = [];
    for (const raw of items) {
        if (typeof raw !== "string" || raw.trim() === "") {
            continue;
        }
        const isDir = raw.endsWith("/") || raw.endsWith("\\");
        const clean = cleanPath(raw);
        result.push({
            path: clean,
            name: getEntryName(raw),
            isDir,
        });
    }
    result.sort((a, b) => {
        if (a.isDir !== b.isDir) {
            return a.isDir ? -1 : 1;
        }
        return a.name.localeCompare(b.name, "zh-CN");
    });
    return result;
};

const setBusy = (isBusy, label) => {
    busy.value = isBusy;
    if (label) {
        output.value = label;
    }
};

const appendOutput = (text) => {
    output.value = text;
};

const errorOutput = (message) => {
    output.value = `错误：${message}`;
};

const fetchDirectory = async (prefix) => {
    if (browserController) {
        browserController.abort();
    }
    browserController = new AbortController();

    const url = new URL("/api/path", window.location.origin);
    if (prefix !== "") {
        url.searchParams.set("prefix", prefix);
    }

    const res = await fetch(url.toString(), { signal: browserController.signal });
    const data = await res.json();
    if (!res.ok || !data.ok || !Array.isArray(data.items)) {
        throw new Error(data.error || "读取路径失败。");
    }
    return data;
};

const loadDirectory = async (dir) => {
    browserLoading.value = true;
    browserError.value = "";
    try {
        const prefix = dir ? withTrailingSeparator(dir) : "";
        const data = await fetchDirectory(prefix);
        if (typeof data.root === "string" && data.root !== "") {
            browserRoot.value = cleanPath(data.root);
        }
        browserEntries.value = buildEntries(data.items);
        if (dir && dir !== "") {
            browserDir.value = cleanPath(dir);
        } else if (browserRoot.value !== "") {
            browserDir.value = browserRoot.value;
        }
    } catch (err) {
        if (err && err.name === "AbortError") {
            return;
        }
        browserError.value = err && err.message ? err.message : "读取路径失败。";
        browserEntries.value = [];
    } finally {
        browserLoading.value = false;
    }
};

const parentDirectory = (dir) => {
    const normalized = cleanPath(dir);
    if (normalized === "" || normalized === "/") {
        return normalized;
    }
    const slash = Math.max(normalized.lastIndexOf("/"), normalized.lastIndexOf("\\"));
    if (slash <= 0) {
        return browserRoot.value || normalized;
    }
    return normalized.slice(0, slash);
};

const openPicker = async () => {
    pickerOpen.value = true;
    let target = browserDir.value || browserRoot.value;
    try {
        const selected = path.value.trim();
        if (selected !== "") {
            const data = await fetchDirectory(selected);
            if (typeof data.root === "string" && data.root !== "") {
                browserRoot.value = cleanPath(data.root);
            }
            if (data.ok) {
                const cleaned = cleanPath(selected);
                const hasDirSelf = data.items.some((item) => item === `${cleaned}/` || item === `${cleaned}\\`);
                target = hasDirSelf ? cleaned : parentDirectory(cleaned);
            }
        }
    } catch (err) {
        const selected = path.value.trim();
        if (selected !== "") {
            target = parentDirectory(selected);
        }
    }

    await loadDirectory(target || "");
};

const closePicker = () => {
    pickerOpen.value = false;
};

const enterDir = async (value) => {
    await loadDirectory(value);
};

const choosePath = (value) => {
    path.value = value;
    pickerOpen.value = false;
};

const chooseCurrentDir = () => {
    if (browserDir.value === "") {
        return;
    }
    path.value = browserDir.value;
    pickerOpen.value = false;
};

const navigateUp = async () => {
    if (!browserDir.value) {
        await loadDirectory(browserRoot.value || "");
        return;
    }
    let parent = parentDirectory(browserDir.value);
    const root = normalizeComparePath(browserRoot.value);
    if (root !== "" && normalizeComparePath(parent).length < root.length) {
        parent = browserRoot.value;
    }
    await loadDirectory(parent || browserRoot.value || "");
};

const refreshBrowser = async () => {
    await loadDirectory(browserDir.value || browserRoot.value || "");
};

const postForm = async (url) => {
    const form = new FormData();
    const value = path.value.trim();
    if (value !== "") {
        form.append("path", value);
    }
    return fetch(url, { method: "POST", body: form });
};

const runInfo = async (url, label) => {
    if (!hasInput()) {
        errorOutput("请先选择媒体路径。");
        return;
    }
    try {
        setBusy(true, `${label} 生成中...`);
        const res = await postForm(url);
        let data = {};
        try {
            data = await res.json();
        } catch (err) {
            data = {};
        }
        if (!res.ok || !data.ok) {
            throw new Error(data.error || "请求失败。");
        }
        appendOutput(data.output || "没有输出。");
    } catch (err) {
        errorOutput(err && err.message ? err.message : "请求失败。");
    } finally {
        setBusy(false);
    }
};

const downloadShots = async () => {
    if (!hasInput()) {
        errorOutput("请先选择媒体路径。");
        return;
    }
    try {
        setBusy(true, "正在生成截图...");
        const res = await postForm("/api/screenshots");
        const contentType = res.headers.get("content-type") || "";
        if (!res.ok || !contentType.includes("application/zip")) {
            let data = {};
            try {
                data = await res.json();
            } catch (err) {
                data = {};
            }
            throw new Error(data.error || "截图请求失败。");
        }
        const blob = await res.blob();
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = "screenshots.zip";
        document.body.appendChild(a);
        a.click();
        a.remove();
        window.URL.revokeObjectURL(url);
        appendOutput("截图已下载为 screenshots.zip。");
    } catch (err) {
        errorOutput(err && err.message ? err.message : "截图请求失败。");
    } finally {
        setBusy(false);
    }
};

const clearOutput = () => {
    if (busy.value) {
        return;
    }
    appendOutput("就绪。");
};

const copyOutput = async () => {
    const text = output.value || "";
    if (text.trim() === "") {
        errorOutput("没有可复制的内容。");
        return;
    }

    try {
        await navigator.clipboard.writeText(text);
    } catch (err) {
        const textarea = document.createElement("textarea");
        textarea.value = text;
        textarea.setAttribute("readonly", "");
        textarea.style.position = "absolute";
        textarea.style.left = "-9999px";
        document.body.appendChild(textarea);
        textarea.select();
        try {
            document.execCommand("copy");
        } finally {
            textarea.remove();
        }
    }

    const original = copyLabel.value;
    copyLabel.value = "已复制";
    setTimeout(() => {
        copyLabel.value = original;
    }, 1200);
};

onBeforeUnmount(() => {
    if (browserController) {
        browserController.abort();
    }
});
</script>
