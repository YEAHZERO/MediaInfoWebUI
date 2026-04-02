export async function fetchDirectory(prefix = "", signal) {
    const url = new URL("/api/path", window.location.origin);
    if (prefix !== "") {
        url.searchParams.set("prefix", prefix);
    }

    const response = await fetch(url.toString(), { signal });
    const data = await response.json();
    if (!response.ok || !data.ok || !Array.isArray(data.items)) {
        throw new Error(data.error || "读取路径失败。");
    }
    return data;
}

export async function requestInfo(path, url, fields = {}) {
    const response = await postForm(url, { path, ...fields });
    const data = await safeReadJSON(response);
    if (!response.ok || !data.ok) {
        throw new Error(data.error || "请求失败。");
    }
    return data;
}

export async function prepareScreenshotZipDownload(path, variant, subtitleMode, count) {
    const response = await postForm("/api/screenshots", { path, mode: "zip", variant, subtitle_mode: subtitleMode, count, prepare_download: "1" });
    const data = await safeReadJSON(response);
    if (!response.ok || !data.ok || typeof data.output !== "string" || data.output.trim() === "") {
        throw buildResponseError(data.error || "截图请求失败。", data);
    }
    return {
        downloadURL: new URL(data.output, window.location.origin).toString(),
        logs: typeof data.logs === "string" ? data.logs : "",
    };
}

export function startPreparedDownload(url) {
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.style.display = "none";
    document.body.appendChild(anchor);
    anchor.click();
    anchor.remove();
}

export async function requestScreenshotLinks(path, variant, subtitleMode, count) {
    const response = await postForm("/api/screenshots", { path, mode: "links", variant, subtitle_mode: subtitleMode, count });
    const data = await safeReadJSON(response);
    if (!response.ok || !data.ok) {
        throw buildResponseError(data.error || "图床链接请求失败。", data);
    }
    return data;
}

export async function fetchBDInfoPlaylists(path) {
    const response = await postForm("/api/bdinfo/playlists", { path });
    const data = await safeReadJSON(response);
    if (!response.ok) {
        throw new Error(data.error || "获取 Playlist 列表失败。");
    }
    return data;
}

export async function createBDInfoJob(path, scanMode = "auto", playlists = []) {
    const fields = { path, scanMode };
    if (playlists.length > 0) {
        fields.playlists = playlists.join(",");
    }
    const response = await postForm("/api/bdinfo/job/create", fields);
    const data = await safeReadJSON(response);
    if (!response.ok || !data.ok) {
        throw new Error(data.error || "创建任务失败。");
    }
    return data.job;
}

export async function fetchBDInfoJobs() {
    const response = await fetch("/api/bdinfo/jobs");
    const data = await safeReadJSON(response);
    if (!response.ok || !data.ok) {
        throw new Error(data.error || "获取任务列表失败。");
    }
    return data.jobs || [];
}

export async function fetchBDInfoJob(jobId) {
    const url = new URL("/api/bdinfo/job", window.location.origin);
    url.searchParams.set("id", jobId);
    const response = await fetch(url.toString());
    const data = await safeReadJSON(response);
    if (!response.ok || !data.ok) {
        throw new Error(data.error || "获取任务详情失败。");
    }
    return data.job;
}

export async function fetchBDInfoReport(jobId) {
    const url = new URL("/api/bdinfo/report", window.location.origin);
    url.searchParams.set("id", jobId);
    const response = await fetch(url.toString());
    const data = await safeReadJSON(response);
    if (!response.ok || !data.ok) {
        throw new Error(data.error || "获取报告失败。");
    }
    return data.report;
}

export function createBDInfoWebSocket(onMessage, onError) {
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const wsUrl = `${protocol}//${window.location.host}/api/bdinfo/ws`;
    const ws = new WebSocket(wsUrl);

    ws.onmessage = (event) => {
        try {
            const msg = JSON.parse(event.data);
            onMessage(msg);
        } catch (e) {
            console.error("Failed to parse WebSocket message:", e);
        }
    };

    ws.onerror = (error) => {
        console.error("WebSocket error:", error);
        if (onError) {
            onError(error);
        }
    };

    return ws;
}

async function postForm(url, fields = {}) {
    const form = new FormData();
    for (const [key, value] of Object.entries(fields)) {
        if (value !== undefined && value !== null && `${value}` !== "") {
            form.append(key, `${value}`);
        }
    }
    return fetch(url, { method: "POST", body: form });
}

async function safeReadJSON(response) {
    try {
        return await response.json();
    } catch {
        return {};
    }
}

function buildResponseError(message, data = {}) {
    const error = new Error(message);
    if (typeof data.logs === "string" && data.logs.trim() !== "") {
        error.logs = data.logs;
    }
    return error;
}
