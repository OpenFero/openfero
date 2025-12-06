<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { AlertCard } from '@/components'
import { useAlertsStore } from '@/stores'
import { useWebSocket } from '@/composables'

const alertsStore = useAlertsStore()
const searchQuery = ref('')
const expandedAlerts = ref<Set<number>>(new Set([0])) // First alert expanded by default

// WebSocket connection for real-time updates
const { connect, isConnected } = useWebSocket('/api/ws', {
    onMessage: (message) => {
        if (message.type === 'alert') {
            // Refresh alerts when a new alert is received
            alertsStore.fetch()
        }
    },
    onConnect: () => {
        console.log('WebSocket connected, fetching initial alerts')
    },
})

// Filtered alerts based on search
const filteredAlerts = computed(() => {
    if (!searchQuery.value.trim()) {
        return alertsStore.alerts
    }

    const query = searchQuery.value.toLowerCase()
    return alertsStore.alerts.filter((entry) => {
        const alertName = entry.alert.labels.alertname?.toLowerCase() || ''
        const status = entry.status.toLowerCase()

        // Search in labels
        const labelsMatch = Object.entries(entry.alert.labels).some(
            ([key, value]) =>
                key.toLowerCase().includes(query) || String(value).toLowerCase().includes(query),
        )

        // Search in annotations
        const annotationsMatch = Object.entries(entry.alert.annotations || {}).some(
            ([key, value]) =>
                key.toLowerCase().includes(query) || String(value).toLowerCase().includes(query),
        )

        return alertName.includes(query) || status.includes(query) || labelsMatch || annotationsMatch
    })
})

const toggleAlert = (index: number) => {
    if (expandedAlerts.value.has(index)) {
        expandedAlerts.value.delete(index)
    } else {
        expandedAlerts.value.add(index)
    }
}

const isExpanded = (index: number) => expandedAlerts.value.has(index)

const expandAll = () => {
    filteredAlerts.value.forEach((_, index) => {
        expandedAlerts.value.add(index)
    })
}

const collapseAll = () => {
    expandedAlerts.value.clear()
}

onMounted(() => {
    alertsStore.fetch()
    connect()
})

// Note: disconnect is handled automatically by useWebSocket's onUnmounted
</script>

<template>
    <section class="px-4 pt-4" id="alerts">
        <!-- Search and controls -->
        <div class="flex flex-col md:flex-row md:items-center md:justify-between gap-3 mb-4">
            <div class="flex-1 max-w-md">
                <div class="relative">
                    <svg class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" fill="none"
                        stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                            d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                    </svg>
                    <input v-model="searchQuery" type="text" class="input pl-10 pr-10" placeholder="Search alerts..."
                        id="search-alerts" name="search" aria-label="Search alerts" />
                    <button v-if="searchQuery"
                        class="absolute right-2 top-1/2 -translate-y-1/2 p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
                        type="button" @click="searchQuery = ''">
                        <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                            <path fill-rule="evenodd"
                                d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z"
                                clip-rule="evenodd" />
                        </svg>
                    </button>
                </div>
            </div>
            <div class="flex items-center gap-3">
                <span class="status-indicator">
                    <span :class="isConnected ? 'status-live' : 'status-disconnected'" class="status-dot"></span>
                    <span
                        :class="isConnected ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'">
                        {{ isConnected ? 'Live' : 'Disconnected' }}
                    </span>
                </span>
                <div class="flex gap-1">
                    <button class="btn btn-secondary btn-sm" @click="expandAll">
                        <svg class="w-4 h-4 mr-1 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                                d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
                        </svg>
                        Expand All
                    </button>
                    <button class="btn btn-secondary btn-sm" @click="collapseAll">
                        <svg class="w-4 h-4 mr-1 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                                d="M8 20l-5-5m0 0h4m-4 0v-4m16-1V4m0 0h-4m4 0v4m0-4l-5 5M4 4l5 5" />
                        </svg>
                        Collapse All
                    </button>
                </div>
            </div>
        </div>

        <!-- Loading state -->
        <div v-if="alertsStore.isLoading" class="text-center py-12">
            <div
                class="inline-block w-8 h-8 border-4 border-primary-500 border-t-transparent rounded-full animate-spin">
            </div>
            <p class="mt-4 text-gray-500 dark:text-gray-400">Loading alerts...</p>
        </div>

        <!-- Backend unavailable state -->
        <div v-else-if="alertsStore.isBackendUnavailable" class="text-center py-12">
            <div class="max-w-sm mx-auto p-6 card">
                <svg class="w-12 h-12 mx-auto text-amber-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                        d="M3 15a4 4 0 004 4h9a5 5 0 10-.1-9.999 5.002 5.002 0 10-9.78 2.096A4.001 4.001 0 003 15z" />
                </svg>
                <h4 class="mt-4 text-lg font-semibold text-gray-900 dark:text-white">Backend Unavailable</h4>
                <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">{{ alertsStore.error }}</p>
                <p class="mt-1 text-xs text-gray-400 dark:text-gray-500">
                    Make sure the OpenFero backend is running and accessible.
                </p>
                <button class="btn btn-primary mt-4" @click="alertsStore.fetch()">
                    <svg class="w-4 h-4 mr-1 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                            d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                    </svg>
                    Try Again
                </button>
            </div>
        </div>

        <!-- Error state -->
        <div v-else-if="alertsStore.error"
            class="p-4 rounded-lg bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-400"
            role="alert">
            <div class="flex items-center gap-2">
                <svg class="w-5 h-5 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd"
                        d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z"
                        clip-rule="evenodd" />
                </svg>
                <span>{{ alertsStore.error }}</span>
                <button class="btn btn-danger btn-sm ml-auto" @click="alertsStore.fetch()">
                    Retry
                </button>
            </div>
        </div>

        <!-- Alerts list -->
        <div v-else class="space-y-3">
            <AlertCard v-for="(alert, index) in filteredAlerts"
                :key="`${alert.alert.labels.alertname}-${alert.timestamp}`" :alert="alert" :index="index"
                :expanded="isExpanded(index)" @toggle="toggleAlert" />

            <!-- Empty state -->
            <div v-if="filteredAlerts.length === 0" class="text-center py-12">
                <svg class="w-16 h-16 mx-auto text-gray-300 dark:text-gray-600" fill="none" stroke="currentColor"
                    viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5"
                        d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4" />
                </svg>
                <p class="mt-4 text-lg text-gray-500 dark:text-gray-400">
                    {{ searchQuery ? 'No alerts match your search.' : 'No alerts found.' }}
                </p>
                <p v-if="!searchQuery" class="mt-1 text-sm text-gray-400 dark:text-gray-500">
                    Alerts will appear here when Alertmanager sends webhooks to OpenFero.
                </p>
            </div>
        </div>
    </section>
</template>

<style scoped>
#alerts {
    padding-top: 1rem;
}
</style>
