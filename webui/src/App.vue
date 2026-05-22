<template>
    <div class="grain"></div>
    <main class="shell">
        <NoticeToast :text="noticeText" />
        <AppHeader :version="versionInfo" />

        <section class="panel">
            <PathBrowser
                v-model:path="path"
                v-model:search-keyword="searchKeyword"
                :busy="busy"
                :browser-dir="browserDir"
                :browser-error="browserError"
                :browser-loading="browserLoading"
                :can-navigate-up="canNavigateUp"
                :entries="filteredEntries"
                @navigate-up="navigateUp"
                @refresh="refreshBrowser"
                @open-entry="handleEntryDoubleClick"
            />

            <div class="panel-section">
                <div class="panel-section-header">
                    <label>配置</label>
                </div>
                <div class="config-grid">
                    <div class="field-pair">
                        <div class="pair-labels">
                            <label class="field-label-muted">生成 BDInfo</label>
                            <label class="field-label-muted">BDInfo 高级</label>
                        </div>
                        <div class="pair-content">
                            <BDInfoOutputPicker v-model="bdinfoMode" :busy="busy" />
                            <BDInfoPanel
                                ref="bdinfoPanelRef"
                                :path="path"
                                :has-input="hasInput"
                                :busy="busy"
                                :scan-mode="bdinfoScanMode"
                                @notice="showNotice"
                                @busy-change="handleBDInfoBusyChange"
                            />
                        </div>
                    </div>
                    <div class="field-pair">
                        <div class="pair-labels">
                            <label class="field-label-muted">截图模式</label>
                            <label class="field-label-muted">扫描模式</label>
                        </div>
                        <div class="pair-content">
                            <ScreenshotVariantPicker v-model="screenshotVariant" :busy="busy" />
                            <BDInfoScanModePicker v-model="bdinfoScanMode" :busy="busy" />
                        </div>
                    </div>
                    <div class="field-pair">
                        <div class="pair-labels">
                            <label class="field-label-muted">字幕处理</label>
                            <label class="field-label-muted">开始扫描</label>
                        </div>
                        <div class="pair-content">
                            <ScreenshotSubtitleModePicker v-model="screenshotSubtitleMode" :busy="busy" />
                            <button
                                type="button"
                                class="scan-btn-compact"
                                :class="{ loading: bdinfoPanelRef?.loading }"
                                :disabled="busy || bdinfoPanelRef?.loading || !hasInput"
                                @click="bdinfoPanelRef?.startScan()"
                            >
                                <span v-if="bdinfoPanelRef?.loading" class="action-btn-spinner"></span>
                                <span>{{ bdinfoPanelRef?.loading ? "创建中..." : "开始扫描" }}</span>
                            </button>
                        </div>
                    </div>
                    <div class="field-pair">
                        <div class="pair-labels">
                            <label for="screenshot-count" class="field-label-muted">截图数量</label>
                            <label class="field-label-muted">图床选择</label>
                        </div>
                        <div class="pair-content">
                            <input
                                id="screenshot-count"
                                class="config-number-input"
                                type="number"
                                inputmode="numeric"
                                min="1"
                                max="10"
                                step="1"
                                :disabled="busy"
                                :value="screenshotCount"
                                @input="handleScreenshotCountInput"
                                @blur="handleScreenshotCountBlur"
                            />
                            <ScreenshotHostPicker v-model="screenshotHost" :busy="busy" />
                        </div>
                    </div>
                </div>
            </div>

            <div class="panel-section panel-section-actions">
                <div class="panel-section-header">
                    <label>操作</label>
                </div>
                <ActionButtons
                        :busy="busy"
                        :active-action="activeAction"
                        :has-input="hasInput"
                        @mediainfo="runInfo('/api/mediainfo', 'MediaInfo', {}, 'mediainfo')"
                        @bdinfo="runInfo('/api/bdinfo', 'BDInfo', { bdinfo_mode: bdinfoMode }, 'bdinfo')"
                        @mkvmerge-tracks="runInfo('/api/mkvmerge/tracks', 'MKVMerge 轨道信息', {}, 'mkvmerge-tracks')"
                        @download-shots="downloadShots"
                        @output-links="outputShotLinks"
                        @download-logs="downloadLogs"
                    />
            </div>
        </section>

        <OutputPanel
            v-if="showOutputPanel"
            :busy="busy"
            :copy-output-label="copyOutputLabel"
            :output-text="outputText"
            :status-message="statusMessage"
            :task-progress="taskProgress"
            @copy="copyOutputText"
            @clear="clearOutputText"
        />

        <ImageLinksPanel
            v-if="showImageLinksPanel"
            :busy="busy"
            :copy-links-label="copyLinksLabel"
            :copy-b-b-code-label="copyBBCodeLabel"
            :link-status-text="linkStatusText"
            :link-items="linkItems"
            :task-progress="taskProgress"
            @append-links="appendShotLinks"
            @copy-links="copyLinks"
            @copy-bbcode="copyBBCode"
            @clear="clearLinkItems"
            @remove-link="removeLink"
        />
    </main>
</template>

<script setup>
import { ref, watch, onMounted } from "vue";
import ActionButtons from "./components/ActionButtons.vue";
import AppHeader from "./components/AppHeader.vue";
import BDInfoOutputPicker from "./components/BDInfoOutputPicker.vue";
import BDInfoPanel from "./components/BDInfoPanel.vue";
import BDInfoScanModePicker from "./components/BDInfoScanModePicker.vue";
import ImageLinksPanel from "./components/ImageLinksPanel.vue";
import NoticeToast from "./components/NoticeToast.vue";
import OutputPanel from "./components/OutputPanel.vue";
import PathBrowser from "./components/PathBrowser.vue";
import ScreenshotSubtitleModePicker from "./components/ScreenshotSubtitleModePicker.vue";
import ScreenshotVariantPicker from "./components/ScreenshotVariantPicker.vue";
import ScreenshotHostPicker from "./components/ScreenshotHostPicker.vue";
import { useMediaActions } from "./composables/useMediaActions";
import { usePathBrowser } from "./composables/usePathBrowser";
import { loadAppState, saveAppState } from "./utils/storage";
import { fetchVersionInfo } from "./api/media";

const persistedState = loadAppState();
const screenshotVariant = ref(persistedState.screenshotVariant);
const screenshotCount = ref(persistedState.screenshotCount || 4);
const screenshotSubtitleMode = ref(persistedState.screenshotSubtitleMode);
const screenshotHost = ref(persistedState.screenshotHost || "pixhost");
const bdinfoMode = ref(persistedState.bdinfoMode);
const bdinfoScanMode = ref(persistedState.bdinfoScanMode || "auto");
const bdinfoPanelRef = ref(null);
const versionInfo = ref(null);
const lastBuildTime = ref(localStorage.getItem('lastBuildTime') || '');

const pathBrowser = usePathBrowser({
    initialPath: persistedState.path,
    initialBrowserDir: persistedState.browserDir,
});
const mediaActions = useMediaActions(pathBrowser.path, screenshotVariant, screenshotCount, screenshotSubtitleMode, screenshotHost, pathBrowser.hasInput);

const {
    path,
    searchKeyword,
    browserDir,
    browserError,
    browserLoading,
    canNavigateUp,
    filteredEntries,
    hasInput,
    navigateUp,
    refreshBrowser,
    handleEntryDoubleClick,
} = pathBrowser;

const {
    outputText,
    linkItems,
    busy,
    activeAction,
    noticeText,
    linkStatusText,
    copyOutputLabel,
    copyLinksLabel,
    copyBBCodeLabel,
    statusMessage,
    showOutputPanel,
    showImageLinksPanel,
    taskProgress,
    runInfo,
    downloadShots,
    outputShotLinks,
    appendShotLinks,
    clearOutputText,
    clearLinkItems,
    copyOutputText,
    copyLinks,
    copyBBCode,
    removeLink,
} = mediaActions;

const bdinfoBusy = ref(false);

const clampScreenshotCount = (value) => {
    const parsed = Number.parseInt(`${value ?? ""}`.trim(), 10);
    if (!Number.isFinite(parsed)) {
        return 4;
    }
    return Math.min(10, Math.max(1, parsed));
};

const handleScreenshotCountInput = (event) => {
    const nextValue = clampScreenshotCount(event?.target?.value);
    screenshotCount.value = nextValue;
    if (event?.target) {
        event.target.value = `${nextValue}`;
    }
};

const handleScreenshotCountBlur = (event) => {
    const nextValue = clampScreenshotCount(event?.target?.value || screenshotCount.value);
    screenshotCount.value = nextValue;
    if (event?.target) {
        event.target.value = `${nextValue}`;
    }
};

const showNotice = (message) => {
    noticeText.value = message;
    setTimeout(() => {
        noticeText.value = "";
    }, 2400);
};

const handleBDInfoBusyChange = (isBusy) => {
    bdinfoBusy.value = isBusy;
};

const downloadLogs = async () => {
    try {
        const response = await fetch("/api/logs/download");
        if (!response.ok) {
            throw new Error("下载日志失败");
        }
        const blob = await response.blob();
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = "mediainfo-logs.txt";
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        window.URL.revokeObjectURL(url);
    } catch (error) {
        showNotice("下载日志失败: " + error.message);
    }
};

const checkBuildVersion = async () => {
    try {
        const version = await fetchVersionInfo();
        if (version) {
            versionInfo.value = version;
            
            if (version.buildTime && version.buildTime !== lastBuildTime.value) {
                lastBuildTime.value = version.buildTime;
                localStorage.setItem('lastBuildTime', version.buildTime);
                showNotice(`检测到新版本: ${version.version} (${new Date(version.buildTime).toLocaleString()})`);
            }
        }
    } catch (e) {
        console.warn("Failed to check build version:", e);
    }
};

onMounted(() => {
    checkBuildVersion();
});

watch(
    [path, browserDir, screenshotVariant, screenshotCount, screenshotSubtitleMode, screenshotHost, bdinfoMode, bdinfoScanMode],
    ([nextPath, nextBrowserDir, nextVariant, nextCount, nextSubtitleMode, nextHost, nextBDInfoMode, nextBDInfoScanMode]) => {
        saveAppState({
            path: nextPath,
            browserDir: nextBrowserDir,
            screenshotVariant: nextVariant,
            screenshotCount: nextCount,
            screenshotSubtitleMode: nextSubtitleMode,
            screenshotHost: nextHost,
            bdinfoMode: nextBDInfoMode,
            bdinfoScanMode: nextBDInfoScanMode,
        });
    },
    { deep: true, immediate: true },
);
</script>
