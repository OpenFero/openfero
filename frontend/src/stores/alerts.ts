import { defineStore } from "pinia";
import { computed, ref, watch } from "vue";
import { fetchAlerts } from "@/api/alerts";
import { ApiError } from "@/api/client";
import type { AlertStoreEntry } from "@/types";
import { useSocketStore } from "./socket";

export const useAlertsStore = defineStore("alerts", () => {
  const alerts = ref<AlertStoreEntry[]>([]);
  const isLoading = ref(false);
  const error = ref<string | null>(null);
  const isBackendUnavailable = ref(false);
  const searchQuery = ref("");
  const socketStore = useSocketStore();

  watch(
    () => socketStore.isConnected,
    (isConnected) => {
      if (isConnected) {
        console.log("AlertsStore: Socket reconnected, refreshing alerts...");
        fetch();
      }
    }
  );

  const firingAlerts = computed(() => alerts.value.filter((a) => a.status === "firing"));

  const resolvedAlerts = computed(() => alerts.value.filter((a) => a.status === "resolved"));

  const filteredAlerts = computed(() => {
    if (!searchQuery.value) return alerts.value;
    const query = searchQuery.value.toLowerCase();
    return alerts.value.filter((entry) => {
      const alertName = entry.alert.labels.alertname || "";
      const labels = Object.entries(entry.alert.labels)
        .map(([k, v]) => `${k}:${v}`)
        .join(" ");
      return alertName.toLowerCase().includes(query) || labels.toLowerCase().includes(query);
    });
  });

  async function fetch(query?: string) {
    isLoading.value = true;
    error.value = null;
    isBackendUnavailable.value = false;
    try {
      alerts.value = await fetchAlerts(query);
    } catch (e) {
      if (e instanceof ApiError) {
        error.value = e.userMessage;
        isBackendUnavailable.value = e.isNetworkError;
      } else {
        error.value = e instanceof Error ? e.message : "Failed to fetch alerts";
      }
      console.error("Failed to fetch alerts:", e);
    } finally {
      isLoading.value = false;
    }
  }

  function setSearchQuery(query: string) {
    searchQuery.value = query;
  }

  function addAlert(alert: AlertStoreEntry) {
    alerts.value.unshift(alert);
  }

  function updateAlertJobStatus(
    alertIndex: number,
    status: "pending" | "running" | "succeeded" | "failed"
  ) {
    const alert = alerts.value[alertIndex];
    if (alert?.jobInfo) {
      alert.jobInfo.status = status;
    }
  }

  return {
    alerts,
    isLoading,
    error,
    isBackendUnavailable,
    searchQuery,
    firingAlerts,
    resolvedAlerts,
    filteredAlerts,
    fetch,
    setSearchQuery,
    addAlert,
    updateAlertJobStatus,
  };
});
