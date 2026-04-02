<template>
    <header class="hero">
        <div>
            <p class="kicker">本地媒体检测</p>
            <h1>minfo</h1>
            <p class="lead">一键生成 MediaInfo / BDInfo，可下载截图或输出图床链接。</p>
        </div>
        <div class="hero-right">
            <div class="header-info">
                <span class="source-project">感谢源项目 mirrorb/minfo</span>
                <span v-if="version" class="version-info">{{ version.version }} ({{ formatBuildTime(version.buildTime) }})</span>
            </div>
        </div>
    </header>
</template>

<script setup>
import { defineProps } from "vue";

const props = defineProps({
    version: {
        type: Object,
        default: null
    }
});

const formatBuildTime = (buildTime) => {
    if (!buildTime || buildTime === 'unknown') {
        return '未知';
    }
    try {
        return new Date(buildTime).toLocaleDateString();
    } catch {
        return buildTime;
    }
};
</script>

<style scoped>
.header-info {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    gap: 8px;
}

.version-info {
    font-size: 0.8rem;
    color: var(--muted);
    padding: 4px 8px;
    border-radius: 999px;
    background: rgba(47, 111, 109, 0.08);
    border: 1px solid rgba(47, 111, 109, 0.12);
}
</style>
