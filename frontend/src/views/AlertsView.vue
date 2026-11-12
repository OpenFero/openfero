<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue'
import { AlertCard, BaseErrorState, BaseSpinner } from '@/components'
import { useAlertsStore, useSocketStore } from '@/stores'
import type { AlertStoreEntry } from '@/types'

const alertsStore = useAlertsStore()
const socketStore = useSocketStore()
const expandedAlerts = ref<Set<string>>(new Set())
let hasInitializedExpansion = false

function getAlertKey(entry: AlertStoreEntry): string {
  return `${entry.alert.labels.alertname || 'unknown'}-${entry.timestamp}`
}

// Expand the first alert once on initial data load
watch(
  () => alertsStore.alerts,
  (alerts) => {
    if (!hasInitializedExpansion && alerts.length > 0 && alerts[0]) {
      expandedAlerts.value = new Set([getAlertKey(alerts[0])])
      hasInitializedExpansion = true
    }
  },
  { immediate: true },
)

// Listen for real-time updates
let removeListener: (() => void) | null = null

onMounted(() => {
  alertsStore.fetch()
  removeListener = socketStore.addListener((message) => {
    if (message.type === 'alert') {
      alertsStore.fetch()
    }
  })
})

onUnmounted(() => {
  if (removeListener) removeListener()
})

const toggleAlert = (key: string) => {
  const next = new Set(expandedAlerts.value)
  if (next.has(key)) {
    next.delete(key)
  } else {
    next.add(key)
  }
  expandedAlerts.value = next
}

const isExpanded = (key: string) => expandedAlerts.value.has(key)

const expandAll = () => {
  expandedAlerts.value = new Set(alertsStore.filteredAlerts.map(getAlertKey))
}

const collapseAll = () => {
  expandedAlerts.value = new Set()
}
</script>

<template>
  <section
    id="alerts"
    class="px-4 pt-4"
    aria-labelledby="alerts-heading"
  >
    <h1
      id="alerts-heading"
      class="sr-only"
    >
      Alerts
    </h1>
    <!-- Search and controls -->
    <div class="flex flex-col md:flex-row md:items-center md:justify-between gap-3 mb-4">
      <div class="flex-1 max-w-md">
        <div class="relative">
          <svg
            class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
            aria-hidden="true"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
            />
          </svg>
          <input
            id="search-alerts"
            v-model="alertsStore.searchQuery"
            type="text"
            class="input pl-10 pr-10"
            placeholder="Search alerts..."
            name="search"
            aria-label="Search alerts"
          >
          <button
            v-if="alertsStore.searchQuery"
            class="absolute right-2 top-1/2 -translate-y-1/2 p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
            type="button"
            aria-label="Clear search"
            @click="alertsStore.setSearchQuery('')"
          >
            <svg
              class="w-4 h-4"
              fill="currentColor"
              viewBox="0 0 20 20"
              aria-hidden="true"
            >
              <path
                fill-rule="evenodd"
                d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z"
                clip-rule="evenodd"
              />
            </svg>
          </button>
        </div>
      </div>
      <div class="flex items-center gap-3">
        <div class="flex gap-1">
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            @click="expandAll"
          >
            <svg
              class="w-4 h-4 mr-1 inline"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
              aria-hidden="true"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4"
              />
            </svg>
            Expand All
          </button>
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            @click="collapseAll"
          >
            <svg
              class="w-4 h-4 mr-1 inline"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
              aria-hidden="true"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M8 20l-5-5m0 0h4m-4 0v-4m16-1V4m0 0h-4m4 0v4m0-4l-5 5M4 4l5 5"
              />
            </svg>
            Collapse All
          </button>
        </div>
      </div>
    </div>

    <!-- Loading state -->
    <BaseSpinner
      v-if="alertsStore.isLoading"
      message="Loading alerts..."
    />

    <!-- Error / Backend unavailable state -->
    <BaseErrorState
      v-else-if="alertsStore.error"
      :error="alertsStore.error"
      :is-backend-unavailable="alertsStore.isBackendUnavailable"
      @retry="alertsStore.fetch()"
    />

    <!-- Alerts list -->
    <div
      v-else
      class="space-y-3"
    >
      <AlertCard
        v-for="(alert, index) in alertsStore.filteredAlerts"
        :key="getAlertKey(alert)"
        :alert="alert"
        :index="index"
        :expanded="isExpanded(getAlertKey(alert))"
        @toggle="toggleAlert(getAlertKey(alert))"
      />

      <!-- Empty state -->
      <div
        v-if="alertsStore.filteredAlerts.length === 0"
        class="text-center py-12"
      >
        <svg
          class="w-16 h-16 mx-auto text-gray-300 dark:text-gray-600"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
          aria-hidden="true"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="1.5"
            d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4"
          />
        </svg>
        <p class="mt-4 text-lg text-gray-500 dark:text-gray-400">
          {{ alertsStore.searchQuery ? 'No alerts match your search.' : 'No alerts found.' }}
        </p>
        <p
          v-if="!alertsStore.searchQuery"
          class="mt-1 text-sm text-gray-400 dark:text-gray-500"
        >
          Alerts will appear here when Alertmanager sends webhooks to OpenFero.
        </p>
      </div>
    </div>
  </section>
</template>
