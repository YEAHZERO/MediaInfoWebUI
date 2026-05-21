<template>
    <header class="hero">
        <div>
            <p class="kicker">本地媒体检测</p>
            <h1>mediainfo</h1>
            <p class="lead">一键生成 MediaInfo / BDInfo，可下载截图或输出图床链接。</p>
        </div>
        <div class="hero-right">
            <div class="header-info">
                <a
                    class="source-project-link"
                    href="https://github.com/mirrorb/minfo"
                    target="_blank"
                    rel="noreferrer noopener"
                    aria-label="源项目 GitHub 仓库"
                    title="源项目 mirrorb/minfo"
                >
                    <svg viewBox="0 0 16 16" aria-hidden="true">
                        <path
                            d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38
                            0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52
                            -.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.5-1.07
                            -1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12
                            0 0 .67-.21 2.2.82a7.52 7.52 0 0 1 4 0c1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12
                            .51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48
                            0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8Z"
                        />
                    </svg>
                    <span>感谢 mirrorb/minfo</span>
                </a>
                <span v-if="version" class="version-info">{{ formatVersion(version.version) }}</span>
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

const formatVersion = (version) => {
    if (!version) {
        return '';
    }
    // 只显示主版本号（如 v1.5.4），去掉多余的 git 信息
    const match = version.match(/^v?(\d+\.\d+\.\d+)/);
    if (match) {
        return 'v' + match[1];
    }
    return 'v' + version;
};
</script>

<style scoped>
.header-info {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    gap: 8px;
}

.source-project-link {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    color: var(--muted);
    text-decoration: none;
    font-size: 0.85rem;
    transition: color 0.2s;
}

.source-project-link:hover {
    color: var(--text);
}

.source-project-link svg {
    width: 16px;
    height: 16px;
    fill: currentColor;
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
