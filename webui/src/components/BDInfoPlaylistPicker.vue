<template>
    <div class="playlist-picker">
        <div class="playlist-header">
            <span class="playlist-title">Playlists ({{ playlists.length }})</span>
            <div class="playlist-actions">
                <button type="button" class="ghost small" :disabled="busy" @click="$emit('select-all')">全选</button>
                <button type="button" class="ghost small" :disabled="busy" @click="$emit('deselect-all')">清空</button>
                <button
                    type="button"
                    class="ghost small"
                    :disabled="busy || !recommendation || !recommendation.mpls || recommendation.mpls.length === 0"
                    @click="$emit('select-recommended')"
                >
                    推荐
                </button>
            </div>
        </div>

        <div v-if="recommendation" class="playlist-recommendation">
            <span class="recommendation-label">推荐:</span>
            <span class="recommendation-value">
                {{ recommendation.mpls && recommendation.mpls.length > 0 ? recommendation.mpls.join(", ") : "整盘扫描" }}
            </span>
            <span class="recommendation-reason">({{ recommendationReason }})</span>
        </div>

        <div class="playlist-list">
            <div
                v-for="playlist in sortedPlaylists"
                :key="playlist.name"
                class="playlist-item"
                :class="{ selected: isSelected(playlist.name), recommended: isRecommended(playlist.name) }"
                @click="$emit('toggle', playlist.name)"
            >
                <div class="playlist-checkbox">
                    <input type="checkbox" :checked="isSelected(playlist.name)" @click.stop />
                </div>
                <div class="playlist-info">
                    <span class="playlist-name">{{ playlist.name }}</span>
                    <span class="playlist-meta">
                        <span v-if="playlist.lengthSeconds > 0" class="playlist-duration">
                            {{ formatDuration(playlist.lengthSeconds) }}
                        </span>
                        <span v-if="playlist.sizeBytes > 0" class="playlist-size">{{ formatSize(playlist.sizeBytes) }}</span>
                    </span>
                </div>
                <span v-if="playlist.lengthSeconds >= 600" class="playlist-badge">主片</span>
            </div>
        </div>

        <div v-if="playlists.length === 0 && !loading" class="playlist-empty">
            <span>暂无 Playlist 数据</span>
        </div>
    </div>
</template>

<script setup>
import { computed } from "vue";

const props = defineProps({
    playlists: { type: Array, default: () => [] },
    selected: { type: Array, default: () => [] },
    recommendation: { type: Object, default: null },
    busy: { type: Boolean, default: false },
    loading: { type: Boolean, default: false },
});

defineEmits(["toggle", "select-all", "deselect-all", "select-recommended"]);

const sortedPlaylists = computed(() => {
    return [...props.playlists].sort((a, b) => {
        if (a.lengthSeconds !== b.lengthSeconds) {
            return b.lengthSeconds - a.lengthSeconds;
        }
        return a.name.localeCompare(b.name);
    });
});

const recommendationReason = computed(() => {
    if (!props.recommendation) return "";
    const reasons = {
        "largest-size-over-10min": "最大文件，时长 > 10分钟",
        "no-playlists-over-10min": "无超过10分钟的 Playlist",
        whole: "整盘扫描",
        single: "单个 Playlist",
    };
    return reasons[props.recommendation.reason] || props.recommendation.reason;
});

const isSelected = (name) => props.selected.includes(name);

const isRecommended = (name) => {
    if (!props.recommendation || !props.recommendation.mpls) return false;
    return props.recommendation.mpls.includes(name);
};

const formatDuration = (seconds) => {
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    const s = Math.floor(seconds % 60);
    if (h > 0) {
        return `${h}:${m.toString().padStart(2, "0")}:${s.toString().padStart(2, "0")}`;
    }
    return `${m}:${s.toString().padStart(2, "0")}`;
};

const formatSize = (bytes) => {
    const gb = bytes / (1024 * 1024 * 1024);
    if (gb >= 1) {
        return `${gb.toFixed(2)} GB`;
    }
    const mb = bytes / (1024 * 1024);
    return `${mb.toFixed(0)} MB`;
};
</script>
