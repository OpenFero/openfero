import { defineStore } from 'pinia'
import { ref } from 'vue'
import { fetchJobs } from '@/api/alerts'
import { ApiError } from '@/api/client'
import type { JobInfo } from '@/types'

export const useJobsStore = defineStore('jobs', () => {
  const jobs = ref<JobInfo[]>([])
  const isLoading = ref(false)
  const error = ref<string | null>(null)
  const isBackendUnavailable = ref(false)

  async function fetch() {
    isLoading.value = true
    error.value = null
    isBackendUnavailable.value = false
    try {
      jobs.value = await fetchJobs()
    } catch (e) {
      if (e instanceof ApiError) {
        error.value = e.userMessage
        isBackendUnavailable.value = e.isNetworkError
      } else {
        error.value = e instanceof Error ? e.message : 'Failed to fetch jobs'
      }
      console.error('Failed to fetch jobs:', e)
    } finally {
      isLoading.value = false
    }
  }

  return {
    jobs,
    isLoading,
    error,
    isBackendUnavailable,
    fetch,
  }
})
