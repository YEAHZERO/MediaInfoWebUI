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
                    <div class="config-left">
                        <div class="field">
                            <label class="field-label-muted">生成 BDInfo</label>
                            <BDInfoOutputPicker v-model="bdinfoMode" :busy="busy" />
                        </div>
                        <div class="field">
                            <label class="field-label-muted">截图模式</label>
                            <ScreenshotVariantPicker v-model="screenshotVariant" :busy="busy" />
                        </div>
                        <div class="field">
                            <label class="field-label-muted">字幕处理</label>
                            <ScreenshotSubtitleModePicker v-model="screenshotSubtitleMode" :busy="busy" />
                        </div>
                        <div class="field">
                            <label for="screenshot-count" class="field-label-muted">截图数量</label>
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
                        </div>
                    </div>
                    <div class="config-right">
                        <div class="field field-full">
                            <label class="field-label-muted">BDInfo 高级</label>
                            <BDInfoPanel
                                :path="path"
                                :has-input="hasInput"
                                :busy="busy"
                                @notice="showNotice"
                                @busy-change="handleBDInfoBusyChange"
                            />
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
                />
            </div>
        </section>

        <OutputPanel
            v-if="showOutputPanel"
            :busy="busy"
            :copy-output-label="copyOutputLabel"
            :output-text="outputText"
            :status-message="statusMessage"
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
import ImageLinksPanel from "./components/ImageLinksPanel.vue";
import NoticeToast from "./components/NoticeToast.vue";
import OutputPanel from "./components/OutputPanel.vue";
import PathBrowser from "./components/PathBrowser.vue";
import ScreenshotSubtitleModePicker from "./components/ScreenshotSubtitleModePicker.vue";
import ScreenshotVariantPicker from "./components/ScreenshotVariantPicker.vue";
import { useMediaActions } from "./composables/useMediaActions";
import { usePathBrowser } from "./composables/usePathBrowser";
import { loadAppState, saveAppState } from "./utils/storage";
import { fetchVersionInfo } from "./api/media";

const persistedState = loadAppState();
const screenshotVariant = ref(persistedState.screenshotVariant);
const screenshotCount = ref(persistedState.screenshotCount || 4);
const screenshotSubtitleMode = ref(persistedState.screenshotSubtitleMode);
const bdinfoMode = ref(persistedState.bdinfoMode);
const versionInfo = ref(null);
const lastBuildTime = ref(localStorage.getItem('lastBuildTime') || '');

const pathBrowser = usePathBrowser({
    initialPath: persistedState.path,
    initialBrowserDir: persistedState.browserDir,
});
const mediaActions = useMediaActions(pathBrowser.path, screenshotVariant, screenshotCount, screenshotSubtitleMode, pathBrowser.hasInput);

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
    [path, browserDir, screenshotVariant, screenshotCount, screenshotSubtitleMode, bdinfoMode],
    ([nextPath, nextBrowserDir, nextVariant, nextCount, nextSubtitleMode, nextBDInfoMode]) => {
        saveAppState({
            path: nextPath,
            browserDir: nextBrowserDir,
            screenshotVariant: nextVariant,
            screenshotCount: nextCount,
            screenshotSubtitleMode: nextSubtitleMode,
            bdinfoMode: nextBDInfoMode,
        });
    },
    { deep: true, immediate: true },
);
</script>
