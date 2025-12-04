import { ref } from 'vue'
import { defineStore } from 'pinia'
import { fetchBuildInfo } from '@/api/app'
import type { BuildInfo } from '@/types'

export const useAppStore = defineStore('app', () => {
  const buildInfo = ref<BuildInfo | null>(null)
  const isLoading = ref(false)

  async function fetchInfo() {
    isLoading.value = true
    try {
      buildInfo.value = await fetchBuildInfo()
    } catch (e) {
      console.error('Failed to fetch build info:', e)
      // Set default values if API is not available
      buildInfo.value = {
        version: 'dev',
        commit: 'unknown',
        buildDate: 'unknown',
      }
    } finally {
      isLoading.value = false
    }
  }

  return {
    buildInfo,
    isLoading,
    fetchInfo,
  }
})
