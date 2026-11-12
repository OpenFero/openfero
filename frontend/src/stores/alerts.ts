import { defineStore } from 'pinia'
import { computed, ref, shallowRef, watch } from 'vue'
import { fetchAlerts } from '@/api/alerts'
import { ApiError } from '@/api/client'
import type { AlertStoreEntry } from '@/types'
import { useSocketStore } from './socket'

export const useAlertsStore = defineStore('alerts', () => {
  const alerts = shallowRef<AlertStoreEntry[]>([])
  const isLoading = ref(false)
  const error = ref<string | null>(null)
  const isBackendUnavailable = ref(false)
  const searchQuery = ref('')
  const socketStore = useSocketStore()

  watch(
    () => socketStore.isConnected,
    (isConnected) => {
      if (isConnected) {
        fetch()
      }
    },
  )

  const firingAlerts = computed(() => alerts.value.filter((a) => a.status === 'firing'))

  const resolvedAlerts = computed(() => alerts.value.filter((a) => a.status === 'resolved'))

  const filteredAlerts = computed(() => {
    const trimmed = searchQuery.value.trim()
    if (!trimmed) return alerts.value

    const query = trimmed.toLowerCase()
    return alerts.value.filter((entry) => {
      const alertName = entry.alert.labels.alertname?.toLowerCase() || ''
      const status = entry.status.toLowerCase()

      const labelsMatch = Object.entries(entry.alert.labels).some(
        ([key, value]) =>
          key.toLowerCase().includes(query) || String(value).toLowerCase().includes(query),
      )

      const annotationsMatch = Object.entries(entry.alert.annotations || {}).some(
        ([key, value]) =>
          key.toLowerCase().includes(query) || String(value).toLowerCase().includes(query),
      )

      return alertName.includes(query) || status.includes(query) || labelsMatch || annotationsMatch
    })
  })

  async function fetch(query?: string) {
    isLoading.value = true
    error.value = null
    isBackendUnavailable.value = false
    try {
      alerts.value = await fetchAlerts(query)
    } catch (e) {
      if (e instanceof ApiError) {
        error.value = e.userMessage
        isBackendUnavailable.value = e.isNetworkError
      } else {
        error.value = e instanceof Error ? e.message : 'Failed to fetch alerts'
      }
      console.error('Failed to fetch alerts:', e)
    } finally {
      isLoading.value = false
    }
  }

  function setSearchQuery(query: string) {
    searchQuery.value = query
  }

  function addAlert(alert: AlertStoreEntry) {
    alerts.value = [alert, ...alerts.value]
  }

  function updateAlertJobStatus(
    alertIndex: number,
    status: 'pending' | 'running' | 'succeeded' | 'failed',
  ) {
    const alert = alerts.value[alertIndex]
    if (alert?.jobInfo) {
      alerts.value = alerts.value.map((entry, idx) =>
        idx === alertIndex && entry.jobInfo
          ? { ...entry, jobInfo: { ...entry.jobInfo, status } }
          : entry,
      )
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
  }
})
