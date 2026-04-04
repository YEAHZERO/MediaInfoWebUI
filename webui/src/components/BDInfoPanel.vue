<template>
    <div class="bdinfo-panel">
        <div class="bdinfo-header-row">
            <div class="bdinfo-tabs">
                <button
                    type="button"
                    class="ghost tab-btn"
                    :class="{ active: activeTab === 'scan' }"
                    :disabled="busy"
                    @click="activeTab = 'scan'"
                >
                    扫描
                </button>
                <button
                    type="button"
                    class="ghost tab-btn"
                    :class="{ active: activeTab === 'history' }"
                    :disabled="busy"
                    @click="activeTab = 'history'"
                >
                    历史
                </button>
            </div>
            <div v-if="activeTab === 'scan'" class="scan-mode">
                <label class="field-label-muted">扫描模式</label>
                <div class="variant-picker">
                    <button
                        type="button"
                        class="ghost variant-option"
                        :class="{ active: scanMode === 'auto' }"
                        :disabled="busy"
                        @click="scanMode = 'auto'"
                    >
                        自动选择
                    </button>
                    <button
                        type="button"
                        class="ghost variant-option"
                        :class="{ active: scanMode === 'playlists' }"
                        :disabled="busy"
                        @click="handleManualMode"
                    >
                        手动选择
                    </button>
                    <button
                        type="button"
                        class="ghost variant-option"
                        :class="{ active: scanMode === 'whole' }"
                        :disabled="busy"
                        @click="scanMode = 'whole'"
                    >
                        整盘扫描
                    </button>
                </div>
            </div>
        </div>

        <div v-if="activeTab === 'scan'" class="bdinfo-scan">
            <div v-if="scanMode === 'playlists'" class="playlist-section">
                <div class="section-header">
                    <span>选择 Playlists</span>
                    <button type="button" class="ghost small" :disabled="busy || loadingPlaylists" @click="loadPlaylists">
                        {{ loadingPlaylists ? "加载中..." : "加载列表" }}
                    </button>
                </div>
                <BDInfoPlaylistPicker
                    :playlists="playlists"
                    :selected="selectedPlaylists"
                    :recommendation="recommendation"
                    :busy="busy"
                    :loading="loadingPlaylists"
                    @toggle="togglePlaylist"
                    @select-all="selectAllPlaylists"
                    @deselect-all="deselectAllPlaylists"
                    @select-recommended="selectRecommended"
                />
            </div>

            <div class="scan-actions">
                <button
                    type="button"
                    class="action-btn"
                    :class="{ loading: loading }"
                    :disabled="busy || loading || !hasInput"
                    @click="handleStartScan"
                >
                    <span v-if="loading" class="action-btn-spinner"></span>
                    <span>{{ loading ? "创建中..." : "开始扫描" }}</span>
                </button>
            </div>

            <div v-if="hasActiveJob" class="active-job-section">
                <div class="section-header">当前任务</div>
                <BDInfoJobProgress :job="activeJob" @view-report="handleViewReport" @copy-summary="handleCopySummary" />
            </div>
        </div>

        <div v-if="activeTab === 'history'" class="bdinfo-history">
            <BDInfoJobHistory
                :jobs="sortedJobs"
                :active-job-id="activeJob ? activeJob.id : ''"
                :ws-connected="wsConnected"
                @select="handleSelectJob"
            />

            <div v-if="activeJob" class="selected-job-section">
                <BDInfoJobProgress :job="activeJob" @view-report="handleViewReport" @copy-summary="handleCopySummary" />
            </div>
        </div>

        <div v-if="showReport" class="report-modal" @click.self="showReport = false">
            <div class="report-content">
                <div class="report-header">
                    <span>扫描报告</span>
                    <button type="button" class="ghost small" @click="showReport = false">关闭</button>
                </div>
                <div class="report-body">
                    <pre>{{ reportContent }}</pre>
                </div>
                <div class="report-actions">
                    <button type="button" class="ghost" @click="copyReport">复制报告</button>
                </div>
            </div>
        </div>
    </div>
</template>

<script setup>
import { ref, watch, toRef } from "vue";
import BDInfoJobHistory from "./BDInfoJobHistory.vue";
import BDInfoJobProgress from "./BDInfoJobProgress.vue";
import BDInfoPlaylistPicker from "./BDInfoPlaylistPicker.vue";
import { useBDInfoJobs } from "../composables/useBDInfoJobs";
import { copyText } from "../utils/output";

const props = defineProps({
    path: { type: String, default: "" },
    hasInput: { type: Boolean, default: false },
    busy: { type: Boolean, default: false },
});

const emit = defineEmits(["notice", "busy-change"]);

const activeTab = ref("scan");
const showReport = ref(false);
const reportContent = ref("");

const {
    sortedJobs,
    activeJob,
    playlists,
    recommendation,
    selectedPlaylists,
    scanMode,
    loading,
    loadingPlaylists,
    wsConnected,
    hasActiveJob,
    loadPlaylists,
    startJob,
    selectJob,
    loadReport,
    togglePlaylist,
    selectAllPlaylists,
    deselectAllPlaylists,
    selectRecommended,
} = useBDInfoJobs(toRef(props, 'path'), toRef(props, 'hasInput'));

watch(loading, (val) => {
    emit("busy-change", val);
});

const handleManualMode = async () => {
    scanMode.value = "playlists";
    if (playlists.value.length === 0) {
        await loadPlaylists();
    }
};

const handleStartScan = async () => {
    const job = await startJob();
    if (job) {
        emit("notice", "任务已创建");
        activeTab.value = "history";
    }
};

const handleSelectJob = async (jobId) => {
    await selectJob(jobId);
};

const handleViewReport = async (jobId) => {
    const report = await loadReport(jobId);
    if (report) {
        reportContent.value = report;
        showReport.value = true;
    }
};

const handleCopySummary = async (summary) => {
    if (!summary) {
        emit("notice", "没有可复制的摘要");
        return;
    }
    try {
        await copyText(summary);
        emit("notice", "摘要已复制");
    } catch {
        emit("notice", "复制失败");
    }
};

const copyReport = async () => {
    try {
        await copyText(reportContent.value);
        emit("notice", "报告已复制");
    } catch {
        emit("notice", "复制失败");
    }
};
</script>
