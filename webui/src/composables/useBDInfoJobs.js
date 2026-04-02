import { onMounted, onUnmounted, ref, computed } from "vue";
import { createBDInfoWebSocket, fetchBDInfoJobs, fetchBDInfoJob, fetchBDInfoReport, createBDInfoJob, fetchBDInfoPlaylists } from "../api/media";

export function useBDInfoJobs(path, hasInput) {
    const jobs = ref([]);
    const activeJob = ref(null);
    const playlists = ref([]);
    const recommendation = ref(null);
    const selectedPlaylists = ref([]);
    const scanMode = ref("auto");
    const loading = ref(false);
    const loadingPlaylists = ref(false);
    const error = ref("");
    const ws = ref(null);
    const wsConnected = ref(false);
    const pollingInterval = ref(null);
    const pollingEnabled = ref(false);

    const hasActiveJob = computed(() => {
        if (!activeJob.value) return false;
        return activeJob.value.status === "queued" || activeJob.value.status === "running";
    });

    const sortedJobs = computed(() => {
        return [...jobs.value].sort((a, b) => {
            if (a.status === "running" && b.status !== "running") return -1;
            if (a.status !== "running" && b.status === "running") return 1;
            if (a.status === "queued" && b.status !== "queued") return -1;
            if (a.status !== "queued" && b.status === "queued") return 1;
            return new Date(b.startTime) - new Date(a.startTime);
        });
    });

    const startPolling = () => {
        if (pollingInterval.value) {
            clearInterval(pollingInterval.value);
        }
        
        pollingEnabled.value = true;
        pollingInterval.value = setInterval(async () => {
            if (!wsConnected.value) {
                await loadJobs();
                if (activeJob.value) {
                    try {
                        const updatedJob = await fetchBDInfoJob(activeJob.value.id);
                        activeJob.value = updatedJob;
                        
                        const jobIndex = jobs.value.findIndex(j => j.id === updatedJob.id);
                        if (jobIndex >= 0) {
                            jobs.value[jobIndex] = updatedJob;
                        }
                    } catch (e) {
                        console.warn("Failed to poll job status:", e);
                    }
                }
            }
        }, 3000);
    };

    const stopPolling = () => {
        if (pollingInterval.value) {
            clearInterval(pollingInterval.value);
            pollingInterval.value = null;
        }
        pollingEnabled.value = false;
    };

    const connectWebSocket = () => {
        try {
            if (ws.value) {
                ws.value.close();
            }

            ws.value = createBDInfoWebSocket(
                (msg) => {
                    if (msg.type === "job_update" && msg.data) {
                        updateJobFromWS(msg.data);
                    } else if (msg.type === "progress" && msg.data) {
                        updateJobProgress(msg.data);
                    }
                    wsConnected.value = true;
                    if (pollingEnabled.value) {
                        stopPolling();
                    }
                },
                () => {
                    wsConnected.value = false;
                    if (!pollingEnabled.value) {
                        startPolling();
                    }
                    setTimeout(connectWebSocket, 5000);
                }
            );

            ws.value.onopen = () => {
                wsConnected.value = true;
                if (pollingEnabled.value) {
                    stopPolling();
                }
            };

            ws.value.onclose = () => {
                wsConnected.value = false;
                if (!pollingEnabled.value) {
                    startPolling();
                }
                setTimeout(connectWebSocket, 5000);
            };
        } catch (e) {
            console.error("WebSocket connection failed:", e);
            wsConnected.value = false;
            if (!pollingEnabled.value) {
                startPolling();
            }
        }
    };

    const updateJobFromWS = (jobData) => {
        const index = jobs.value.findIndex((j) => j.id === jobData.id);
        if (index >= 0) {
            jobs.value[index] = jobData;
        } else {
            jobs.value.unshift(jobData);
        }

        if (activeJob.value && activeJob.value.id === jobData.id) {
            activeJob.value = jobData;
        }

        if (jobData.status === "running") {
            activeJob.value = jobData;
        }
    };

    const updateJobProgress = (progressData) => {
        const job = jobs.value.find((j) => j.id === progressData.jobId);
        if (job) {
            job.progress = progressData.progress;
            job.etaSec = progressData.etaSec;
        }
        if (activeJob.value && activeJob.value.id === progressData.jobId) {
            activeJob.value.progress = progressData.progress;
            activeJob.value.etaSec = progressData.etaSec;
        }
    };

    const loadJobs = async () => {
        try {
            loading.value = true;
            jobs.value = await fetchBDInfoJobs();
        } catch (e) {
            error.value = e.message;
        } finally {
            loading.value = false;
        }
    };

    const loadPlaylists = async () => {
        if (!hasInput.value) {
            return;
        }
        try {
            loadingPlaylists.value = true;
            error.value = "";
            const data = await fetchBDInfoPlaylists(path.value.trim());
            playlists.value = data.playlists || [];
            recommendation.value = data.recommendation || null;

            if (recommendation.value && recommendation.value.mpls) {
                selectedPlaylists.value = [...recommendation.value.mpls];
            }
        } catch (e) {
            error.value = e.message;
            playlists.value = [];
            recommendation.value = null;
        } finally {
            loadingPlaylists.value = false;
        }
    };

    const startJob = async (customScanMode, customPlaylists) => {
        if (!hasInput.value) {
            error.value = "请先选择媒体路径。";
            return null;
        }

        try {
            loading.value = true;
            error.value = "";
            const mode = customScanMode || scanMode.value;
            const mpls = customPlaylists || selectedPlaylists.value;
            const job = await createBDInfoJob(path.value.trim(), mode, mpls);
            activeJob.value = job;
            jobs.value.unshift(job);
            return job;
        } catch (e) {
            error.value = e.message;
            return null;
        } finally {
            loading.value = false;
        }
    };

    const selectJob = async (jobId) => {
        try {
            loading.value = true;
            const job = await fetchBDInfoJob(jobId);
            activeJob.value = job;
            return job;
        } catch (e) {
            error.value = e.message;
            return null;
        } finally {
            loading.value = false;
        }
    };

    const loadReport = async (jobId) => {
        try {
            loading.value = true;
            return await fetchBDInfoReport(jobId);
        } catch (e) {
            error.value = e.message;
            return null;
        } finally {
            loading.value = false;
        }
    };

    const clearActiveJob = () => {
        activeJob.value = null;
    };

    const togglePlaylist = (mpls) => {
        const index = selectedPlaylists.value.indexOf(mpls);
        if (index >= 0) {
            selectedPlaylists.value.splice(index, 1);
        } else {
            selectedPlaylists.value.push(mpls);
        }
    };

    const selectAllPlaylists = () => {
        selectedPlaylists.value = playlists.value.map((p) => p.name);
    };

    const deselectAllPlaylists = () => {
        selectedPlaylists.value = [];
    };

    const selectRecommended = () => {
        if (recommendation.value && recommendation.value.mpls) {
            selectedPlaylists.value = [...recommendation.value.mpls];
        }
    };

    onMounted(() => {
        loadJobs();
        connectWebSocket();
        startPolling();
    });

    onUnmounted(() => {
        if (ws.value) {
            ws.value.close();
        }
        stopPolling();
    });

    return {
        jobs,
        sortedJobs,
        activeJob,
        playlists,
        recommendation,
        selectedPlaylists,
        scanMode,
        loading,
        loadingPlaylists,
        error,
        wsConnected,
        pollingEnabled,
        hasActiveJob,
        loadJobs,
        loadPlaylists,
        startJob,
        selectJob,
        loadReport,
        clearActiveJob,
        togglePlaylist,
        selectAllPlaylists,
        deselectAllPlaylists,
        selectRecommended,
    };
}
