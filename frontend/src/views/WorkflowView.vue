<script setup lang="ts">
import { onMounted, onUnmounted, computed, ref } from 'vue'
import { VueFlow, useVueFlow } from '@vue-flow/core'
import { Background } from '@vue-flow/background'
import { Controls } from '@vue-flow/controls'
import type { Node, Edge } from '@vue-flow/core'
import { useAlertsStore } from '@/stores'
import { useWebSocket } from '@/composables'

const alertsStore = useAlertsStore()
const selectedNode = ref<Node | null>(null)

// WebSocket for real-time updates
const { isConnected, connect, disconnect } = useWebSocket('/api/ws', {
  onMessage: (message) => {
    if (message.type === 'alert' || message.type === 'job_status') {
      alertsStore.fetch()
    }
  },
  reconnectInterval: 3000,
  maxReconnectAttempts: 10,
})

const { fitView } = useVueFlow()

// Transform alerts to workflow nodes and edges
const nodes = computed<Node[]>(() => {
  const result: Node[] = []
  let yOffset = 50

  alertsStore.alerts.forEach((entry, index) => {
    const alertName = entry.alert.labels.alertname || 'Unknown'

    // Alert node
    result.push({
      id: `alert-${index}`,
      type: 'alert',
      position: { x: 50, y: yOffset },
      data: {
        label: alertName,
        status: entry.status,
        timestamp: entry.timestamp,
        severity: entry.alert.labels.severity || 'warning',
        description: entry.alert.annotations?.description || entry.alert.annotations?.summary || '',
      },
    })

    // Job node (if job was triggered)
    if (entry.jobInfo) {
      result.push({
        id: `job-${index}`,
        type: 'job',
        position: { x: 350, y: yOffset },
        data: {
          label: entry.jobInfo.jobName,
          configMap: entry.jobInfo.configMapName,
          image: entry.jobInfo.image,
          status: entry.jobInfo.status || 'pending',
          startedAt: entry.jobInfo.startedAt,
          completedAt: entry.jobInfo.completedAt,
        },
      })

      // Result node
      result.push({
        id: `result-${index}`,
        type: 'result',
        position: { x: 650, y: yOffset },
        data: {
          label: entry.jobInfo.status || 'pending',
          status: entry.jobInfo.status || 'pending',
        },
      })
    }

    yOffset += 140
  })

  return result
})

const edges = computed<Edge[]>(() => {
  const result: Edge[] = []

  alertsStore.alerts.forEach((entry, index) => {
    if (entry.jobInfo) {
      // Alert -> Job edge
      result.push({
        id: `edge-alert-job-${index}`,
        source: `alert-${index}`,
        target: `job-${index}`,
        animated: entry.status === 'firing',
        style: { stroke: entry.status === 'firing' ? '#ef4444' : '#22c55e', strokeWidth: 2 },
        type: 'smoothstep',
      })

      // Job -> Result edge
      result.push({
        id: `edge-job-result-${index}`,
        source: `job-${index}`,
        target: `result-${index}`,
        animated: entry.jobInfo.status === 'running',
        style: {
          strokeWidth: 2,
          stroke:
            entry.jobInfo.status === 'succeeded'
              ? '#22c55e'
              : entry.jobInfo.status === 'failed'
                ? '#ef4444'
                : '#6b7280',
        },
        type: 'smoothstep',
      })
    }
  })

  return result
})

function handleNodeClick(event: { node: Node }) {
  selectedNode.value = event.node
}

function clearSelection() {
  selectedNode.value = null
}

onMounted(() => {
  alertsStore.fetch()
  connect()
  setTimeout(() => fitView(), 100)
})

onUnmounted(() => {
  disconnect()
})
</script>

<template>
  <div class="flex flex-col h-[calc(100vh-90px)] p-4">
    <!-- Header -->
    <div class="flex justify-between items-center mb-4">
      <h4 class="text-xl font-semibold flex items-center gap-2">
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
            d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
        </svg>
        Workflow Visualization
      </h4>
      <div class="flex items-center gap-4">
        <span class="flex items-center gap-1.5 text-sm">
          <span class="relative flex h-2.5 w-2.5">
            <span v-if="isConnected"
              class="animate-ping absolute inline-flex h-full w-full rounded-full bg-green-400 opacity-75"></span>
            <span class="relative inline-flex rounded-full h-2.5 w-2.5"
              :class="isConnected ? 'bg-green-500' : 'bg-red-500'"></span>
          </span>
          {{ isConnected ? 'Live' : 'Disconnected' }}
        </span>
        <button @click="fitView()"
          class="px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-md hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors flex items-center gap-1.5">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
              d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
          </svg>
          Fit View
        </button>
      </div>
    </div>

    <!-- Legend -->
    <div class="flex flex-wrap gap-2 mb-4">
      <span class="inline-flex items-center gap-1 px-2.5 py-1 text-xs font-medium rounded-full bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400">
        <svg class="w-3 h-3" fill="currentColor" viewBox="0 0 20 20">
          <path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd" />
        </svg>
        Firing
      </span>
      <span class="inline-flex items-center gap-1 px-2.5 py-1 text-xs font-medium rounded-full bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400">
        <svg class="w-3 h-3" fill="currentColor" viewBox="0 0 20 20">
          <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
        </svg>
        Resolved / Succeeded
      </span>
      <span class="inline-flex items-center gap-1 px-2.5 py-1 text-xs font-medium rounded-full bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400">
        <svg class="w-3 h-3" fill="currentColor" viewBox="0 0 20 20">
          <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM9.555 7.168A1 1 0 008 8v4a1 1 0 001.555.832l3-2a1 1 0 000-1.664l-3-2z" clip-rule="evenodd" />
        </svg>
        Running
      </span>
      <span class="inline-flex items-center gap-1 px-2.5 py-1 text-xs font-medium rounded-full bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400">
        <svg class="w-3 h-3" fill="currentColor" viewBox="0 0 20 20">
          <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm1-12a1 1 0 10-2 0v4a1 1 0 00.293.707l2.828 2.829a1 1 0 101.415-1.415L11 9.586V6z" clip-rule="evenodd" />
        </svg>
        Pending
      </span>
      <span class="inline-flex items-center gap-1 px-2.5 py-1 text-xs font-medium rounded-full bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300">
        <svg class="w-3 h-3" fill="currentColor" viewBox="0 0 20 20">
          <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" />
        </svg>
        Failed
      </span>
    </div>

    <!-- Vue Flow Canvas -->
    <div class="flex-1 border border-gray-200 dark:border-gray-700 rounded-lg bg-gray-50 dark:bg-gray-800/50 relative min-h-[400px]">
      <VueFlow 
        :nodes="nodes" 
        :edges="edges" 
        :default-viewport="{ zoom: 1, x: 0, y: 0 }" 
        fit-view-on-init
        @node-click="handleNodeClick"
      >
        <Background />
        <Controls />

        <!-- Custom Alert Node -->
        <template #node-alert="{ data }">
          <div
            class="flex items-center gap-3 px-4 py-3 rounded-lg shadow-sm min-w-[200px] border-2 transition-all"
            :class="{
              'border-red-500 bg-red-50 dark:bg-red-900/20': data.status === 'firing',
              'border-green-500 bg-green-50 dark:bg-green-900/20': data.status === 'resolved',
            }"
          >
            <div class="text-2xl" :class="{
              'text-red-500': data.status === 'firing',
              'text-green-500': data.status === 'resolved',
            }">
              <svg class="w-6 h-6" fill="currentColor" viewBox="0 0 20 20">
                <path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd" />
              </svg>
            </div>
            <div class="flex-1 min-w-0">
              <div class="font-semibold text-sm truncate text-gray-900 dark:text-gray-100">{{ data.label }}</div>
              <div class="text-xs text-gray-500 dark:text-gray-400 capitalize">{{ data.status }}</div>
            </div>
          </div>
        </template>

        <!-- Custom Job Node -->
        <template #node-job="{ data }">
          <div
            class="flex items-center gap-3 px-4 py-3 rounded-lg shadow-sm min-w-[200px] border-2 transition-all bg-white dark:bg-gray-800"
            :class="{
              'border-blue-500': data.status === 'running',
              'border-green-500': data.status === 'succeeded',
              'border-red-500': data.status === 'failed',
              'border-gray-300 dark:border-gray-600': data.status === 'pending',
            }"
          >
            <div class="text-2xl text-blue-500">
              <svg class="w-6 h-6" fill="currentColor" viewBox="0 0 20 20">
                <path fill-rule="evenodd" d="M11.49 3.17c-.38-1.56-2.6-1.56-2.98 0a1.532 1.532 0 01-2.286.948c-1.372-.836-2.942.734-2.106 2.106.54.886.061 2.042-.947 2.287-1.561.379-1.561 2.6 0 2.978a1.532 1.532 0 01.947 2.287c-.836 1.372.734 2.942 2.106 2.106a1.532 1.532 0 012.287.947c.379 1.561 2.6 1.561 2.978 0a1.533 1.533 0 012.287-.947c1.372.836 2.942-.734 2.106-2.106a1.533 1.533 0 01.947-2.287c1.561-.379 1.561-2.6 0-2.978a1.532 1.532 0 01-.947-2.287c.836-1.372-.734-2.942-2.106-2.106a1.532 1.532 0 01-2.287-.947zM10 13a3 3 0 100-6 3 3 0 000 6z" clip-rule="evenodd" />
              </svg>
            </div>
            <div class="flex-1 min-w-0">
              <div class="font-semibold text-sm truncate text-gray-900 dark:text-gray-100">{{ data.label }}</div>
              <div class="text-xs text-gray-500 dark:text-gray-400 truncate">{{ data.configMap }}</div>
            </div>
          </div>
        </template>

        <!-- Custom Result Node -->
        <template #node-result="{ data }">
          <div
            class="flex items-center gap-3 px-4 py-3 rounded-lg shadow-sm min-w-[140px] border-2 transition-all"
            :class="{
              'border-green-500 bg-green-50 dark:bg-green-900/20': data.status === 'succeeded',
              'border-red-500 bg-red-50 dark:bg-red-900/20': data.status === 'failed',
              'border-yellow-400 bg-yellow-50 dark:bg-yellow-900/20': data.status === 'pending',
              'border-blue-500 bg-blue-50 dark:bg-blue-900/20': data.status === 'running',
            }"
          >
            <div class="text-2xl" :class="{
              'text-green-500': data.status === 'succeeded',
              'text-red-500': data.status === 'failed',
              'text-yellow-500': data.status === 'pending',
              'text-blue-500': data.status === 'running',
            }">
              <svg v-if="data.status === 'succeeded'" class="w-6 h-6" fill="currentColor" viewBox="0 0 20 20">
                <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
              </svg>
              <svg v-else-if="data.status === 'failed'" class="w-6 h-6" fill="currentColor" viewBox="0 0 20 20">
                <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" />
              </svg>
              <svg v-else-if="data.status === 'running'" class="w-6 h-6 animate-spin" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
              </svg>
              <svg v-else class="w-6 h-6" fill="currentColor" viewBox="0 0 20 20">
                <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm1-12a1 1 0 10-2 0v4a1 1 0 00.293.707l2.828 2.829a1 1 0 101.415-1.415L11 9.586V6z" clip-rule="evenodd" />
              </svg>
            </div>
            <div class="flex-1 min-w-0">
              <div class="font-semibold text-sm capitalize text-gray-900 dark:text-gray-100">{{ data.label }}</div>
            </div>
          </div>
        </template>
      </VueFlow>

      <!-- Empty state -->
      <div v-if="nodes.length === 0" class="absolute inset-0 flex flex-col items-center justify-center" @click="clearSelection">
        <svg class="w-16 h-16 text-gray-300 dark:text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5"
            d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
        </svg>
        <p class="text-xl text-gray-400 dark:text-gray-500 mt-4">No workflows to display.</p>
        <p class="text-gray-400 dark:text-gray-500 mt-1">
          Workflows will appear here when alerts trigger remediation jobs.
        </p>
      </div>
    </div>

    <!-- Node Detail Panel -->
    <div 
      v-if="selectedNode" 
      class="fixed bottom-4 right-4 w-80 bg-white dark:bg-gray-800 rounded-lg shadow-lg border border-gray-200 dark:border-gray-700 p-4 z-50"
    >
      <div class="flex justify-between items-start mb-3">
        <h5 class="font-semibold text-gray-900 dark:text-gray-100">
          {{ selectedNode.data.label }}
        </h5>
        <button @click="clearSelection" class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">
          <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
            <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd" />
          </svg>
        </button>
      </div>
      <div class="space-y-2 text-sm">
        <div v-if="selectedNode.data.status" class="flex justify-between">
          <span class="text-gray-500 dark:text-gray-400">Status:</span>
          <span class="font-medium capitalize" :class="{
            'text-red-500': selectedNode.data.status === 'firing' || selectedNode.data.status === 'failed',
            'text-green-500': selectedNode.data.status === 'resolved' || selectedNode.data.status === 'succeeded',
            'text-blue-500': selectedNode.data.status === 'running',
            'text-yellow-500': selectedNode.data.status === 'pending',
          }">{{ selectedNode.data.status }}</span>
        </div>
        <div v-if="selectedNode.data.configMap" class="flex justify-between">
          <span class="text-gray-500 dark:text-gray-400">ConfigMap:</span>
          <span class="font-medium text-gray-900 dark:text-gray-100">{{ selectedNode.data.configMap }}</span>
        </div>
        <div v-if="selectedNode.data.image" class="flex justify-between">
          <span class="text-gray-500 dark:text-gray-400">Image:</span>
          <span class="font-medium text-gray-900 dark:text-gray-100 truncate max-w-[180px]">{{ selectedNode.data.image }}</span>
        </div>
        <div v-if="selectedNode.data.timestamp" class="flex justify-between">
          <span class="text-gray-500 dark:text-gray-400">Time:</span>
          <span class="font-medium text-gray-900 dark:text-gray-100">{{ new Date(selectedNode.data.timestamp).toLocaleString() }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style>
/* Import vue-flow styles globally */
@import '@vue-flow/core/dist/style.css';
@import '@vue-flow/core/dist/theme-default.css';
@import '@vue-flow/controls/dist/style.css';

/* Override vue-flow dark mode */
.dark .vue-flow {
  --vf-node-bg: rgb(31 41 55); /* gray-800 */
  --vf-node-color: rgb(243 244 246); /* gray-100 */
  --vf-handle: rgb(107 114 128); /* gray-500 */
  --vf-box-shadow: 0 2px 4px rgba(0, 0, 0, 0.3);
}

.vue-flow__node {
  padding: 0;
  border: none;
  background: transparent;
}

.vue-flow__controls {
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.dark .vue-flow__controls {
  background: rgb(31 41 55); /* gray-800 */
  border-color: rgb(55 65 81); /* gray-700 */
}

.dark .vue-flow__controls-button {
  background: rgb(31 41 55); /* gray-800 */
  color: rgb(209 213 219); /* gray-300 */
  border-color: rgb(55 65 81); /* gray-700 */
}

.dark .vue-flow__controls-button:hover {
  background: rgb(55 65 81); /* gray-700 */
}

.dark .vue-flow__background {
  background: rgb(17 24 39); /* gray-900 */
}
</style>
