<template>
    <div class="job-progress">
        <div class="job-header">
            <span class="job-id">任务 #{{ job.id.slice(-6) }}</span>
            <span class="job-status" :class="statusClass">{{ statusText }}</span>
        </div>

        <div class="job-path">{{ job.path }}</div>

        <div v-if="job.status === 'running' || job.status === 'queued'" class="progress-section">
            <div class="progress-bar">
                <div class="progress-fill" :style="{ width: `${job.progress || 0}%` }"></div>
            </div>
            <div class="progress-info">
                <span class="progress-percent">{{ (job.progress || 0).toFixed(1) }}%</span>
                <span v-if="job.etaSec > 0" class="progress-eta">ETA: {{ formatETA(job.etaSec) }}</span>
            </div>
        </div>

        <div v-if="job.status === 'done'" class="job-summary">
            <div class="summary-header">扫描完成</div>
            <div class="summary-actions">
                <button type="button" class="ghost" @click="$emit('view-report', job.id)">查看报告</button>
                <button type="button" class="ghost" @click="$emit('copy-summary', job.summary)">复制摘要</button>
            </div>
        </div>

        <div v-if="job.status === 'error'" class="job-error">
            <span class="error-label">错误:</span>
            <span class="error-message">{{ job.error }}</span>
        </div>

        <div class="job-meta">
            <span v-if="job.startTime">开始: {{ formatTime(job.startTime) }}</span>
            <span v-if="job.endTime">结束: {{ formatTime(job.endTime) }}</span>
        </div>
    </div>
</template>

<script setup>
const props = defineProps({
    job: { type: Object, required: true },
});

defineEmits(["view-report", "copy-summary"]);

const statusClass = {
    queued: "status-queued",
    running: "status-running",
    done: "status-done",
    error: "status-error",
}[props.job.status] || "status-unknown";

const statusText = {
    queued: "排队中",
    running: "运行中",
    done: "已完成",
    error: "失败",
}[props.job.status] || props.job.status;

const formatETA = (seconds) => {
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    const s = seconds % 60;
    if (h > 0) {
        return `${h}h ${m}m ${s}s`;
    }
    if (m > 0) {
        return `${m}m ${s}s`;
    }
    return `${s}s`;
};

const formatTime = (timeStr) => {
    const date = new Date(timeStr);
    return date.toLocaleString("zh-CN", {
        month: "2-digit",
        day: "2-digit",
        hour: "2-digit",
        minute: "2-digit",
    });
};
</script>
