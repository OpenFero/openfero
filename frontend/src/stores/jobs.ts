import { defineStore } from "pinia";
import { ref, watch } from "vue";
import { fetchJobs } from "@/api/alerts";
import { ApiError } from "@/api/client";
import type { JobInfo } from "@/types";
import { useSocketStore } from "./socket";

export const useJobsStore = defineStore("jobs", () => {
  const jobs = ref<JobInfo[]>([]);
  const isLoading = ref(false);
  const error = ref<string | null>(null);
  const isBackendUnavailable = ref(false);
  const socketStore = useSocketStore();

  watch(
    () => socketStore.isConnected,
    (isConnected) => {
      if (isConnected) {
        console.log("JobsStore: Socket reconnected, refreshing jobs...");
        fetch();
      }
    }
  );

  async function fetch() {
    isLoading.value = true;
    error.value = null;
    isBackendUnavailable.value = false;
    try {
      jobs.value = await fetchJobs();
    } catch (e) {
      if (e instanceof ApiError) {
        error.value = e.userMessage;
        isBackendUnavailable.value = e.isNetworkError;
      } else {
        error.value = e instanceof Error ? e.message : "Failed to fetch jobs";
      }
      console.error("Failed to fetch jobs:", e);
    } finally {
      isLoading.value = false;
    }
  }

  function updateJob(updatedJob: JobInfo) {
    const index = jobs.value.findIndex((j) => j.operariusName === updatedJob.operariusName);
    if (index !== -1) {
      jobs.value[index] = updatedJob;
    } else {
      jobs.value.push(updatedJob);
    }
  }

  return {
    jobs,
    isLoading,
    error,
    isBackendUnavailable,
    fetch,
    updateJob,
  };
});
