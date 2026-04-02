<template>
    <div class="job-history">
        <div class="history-header">
            <span class="history-title">历史任务</span>
            <span class="history-count">({{ jobs.length }})</span>
            <span v-if="wsConnected" class="ws-status connected">实时</span>
            <span v-else class="ws-status disconnected">离线</span>
        </div>

        <div class="history-list">
            <div
                v-for="job in jobs"
                :key="job.id"
                class="history-item"
                :class="{ active: activeJobId === job.id, [getStatusClass(job.status)]: true }"
                @click="$emit('select', job.id)"
            >
                <div class="history-item-main">
                    <span class="history-id">#{{ job.id.slice(-6) }}</span>
                    <span class="history-status">{{ getStatusText(job.status) }}</span>
                </div>
                <div class="history-item-path">{{ truncatePath(job.path) }}</div>
                <div class="history-item-meta">
                    <span v-if="job.progress > 0 && job.status === 'running'" class="history-progress">
                        {{ job.progress.toFixed(0) }}%
                    </span>
                    <span class="history-time">{{ formatTime(job.startTime) }}</span>
                </div>
            </div>
        </div>

        <div v-if="jobs.length === 0" class="history-empty">
            <span>暂无历史任务</span>
        </div>
    </div>
</template>

<script setup>
const props = defineProps({
    jobs: { type: Array, default: () => [] },
    activeJobId: { type: String, default: "" },
    wsConnected: { type: Boolean, default: false },
});

defineEmits(["select"]);

const getStatusClass = (status) => `status-${status}`;

const getStatusText = (status) => ({
    queued: "排队",
    running: "运行",
    done: "完成",
    error: "失败",
}[status] || status);

const truncatePath = (path) => {
    if (!path) return "";
    const parts = path.split("/");
    return parts.length > 3 ? `.../${parts.slice(-2).join("/")}` : path;
};

const formatTime = (timeStr) => {
    if (!timeStr) return "";
    const date = new Date(timeStr);
    return date.toLocaleString("zh-CN", {
        month: "2-digit",
        day: "2-digit",
        hour: "2-digit",
        minute: "2-digit",
    });
};
</script>
