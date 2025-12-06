<script setup lang="ts">
import { computed } from 'vue'
import { useDateTime } from '@/composables/useDateTime'
import type { AlertStoreEntry } from '@/types'

const props = defineProps<{
  alert: AlertStoreEntry
  index: number
  expanded?: boolean
}>()

const emit = defineEmits<{
  toggle: [index: number]
}>()

const { formatDateTime, toISOString } = useDateTime()

const uniqueId = computed(() => `alert-${props.index}`)

const statusColors = computed(() => {
  switch (props.alert.status) {
    case 'firing':
      return 'bg-red-600 hover:bg-red-700'
    case 'resolved':
      return 'bg-green-600 hover:bg-green-700'
    default:
      return 'bg-primary-600 hover:bg-primary-700'
  }
})

const alertName = computed(() => props.alert.alert.labels.alertname || 'Unknown Alert')

const formattedTimestamp = computed(() => {
  return formatDateTime(props.alert.timestamp, {
    includeMilliseconds: true,
    includeTimezone: true,
  })
})

const isoTimestamp = computed(() => {
  return toISOString(props.alert.timestamp)
})

const hasLabels = computed(() => {
  return props.alert.alert.labels && Object.keys(props.alert.alert.labels).length > 0
})

const hasAnnotations = computed(() => {
  return props.alert.alert.annotations && Object.keys(props.alert.alert.annotations).length > 0
})
</script>

<template>
    <div class="accordion-item">
        <h2 :id="`heading${uniqueId}`">
            <button
                class="accordion-button w-full text-left px-4 py-3 font-medium text-white transition-colors flex items-center justify-between"
                :class="[statusColors, { 'rounded-t-lg': true, 'rounded-b-lg': !expanded }]" type="button"
                :aria-expanded="expanded" :aria-controls="`collapse${uniqueId}`" @click="emit('toggle', index)">
                <div class="flex items-center gap-2">
                    <span>{{ alertName }}</span>
                    <span v-if="!alert.jobInfo"
                        class="px-2 py-0.5 text-xs font-medium bg-black/20 text-white rounded border border-white/20"
                        title="No matching Operarius found for this alert">
                        No Remediation
                    </span>
                </div>
                <svg class="w-5 h-5 transition-transform duration-200" :class="{ 'rotate-180': expanded }" fill="none"
                    stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                </svg>
            </button>
        </h2>
        <div v-show="expanded" :id="`collapse${uniqueId}`"
            class="accordion-body bg-white dark:bg-gray-800 rounded-b-lg border border-t-0 border-gray-200 dark:border-gray-700"
            :aria-labelledby="`heading${uniqueId}`">
            <div class="p-4 space-y-4">
                <!-- Metadata Section -->
                <div>
                    <h3 class="flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white mb-2">
                        <svg class="w-4 h-4 text-gray-500" fill="currentColor" viewBox="0 0 20 20">
                            <path fill-rule="evenodd"
                                d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z"
                                clip-rule="evenodd" />
                        </svg>
                        Metadata
                    </h3>
                    <div class="ml-6 space-y-1 text-sm">
                        <div>
                            <strong class="text-gray-700 dark:text-gray-300">Timestamp:</strong>
                            <span class="ml-1 text-gray-600 dark:text-gray-400" :data-timestamp="alert.timestamp"
                                :title="`ISO Format: ${isoTimestamp}`">
                                {{ formattedTimestamp }}
                            </span>
                            <svg class="inline-block w-3 h-3 ml-1 text-gray-400 cursor-help" :title="alert.timestamp"
                                fill="currentColor" viewBox="0 0 20 20">
                                <path fill-rule="evenodd"
                                    d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z"
                                    clip-rule="evenodd" />
                            </svg>
                        </div>
                        <div>
                            <strong class="text-gray-700 dark:text-gray-300">Status:</strong>
                            <span class="ml-1 text-gray-600 dark:text-gray-400">{{ alert.status }}</span>
                        </div>
                    </div>
                </div>

                <!-- Job Info Section -->
                <template v-if="alert.jobInfo">
                    <hr class="border-gray-200 dark:border-gray-700" />
                    <div>
                        <h3 class="flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white mb-2">
                            <svg class="w-4 h-4 text-gray-500" fill="currentColor" viewBox="0 0 20 20">
                                <path fill-rule="evenodd"
                                    d="M11.49 3.17c-.38-1.56-2.6-1.56-2.98 0a1.532 1.532 0 01-2.286.948c-1.372-.836-2.942.734-2.106 2.106.54.886.061 2.042-.947 2.287-1.561.379-1.561 2.6 0 2.978a1.532 1.532 0 01.947 2.287c-.836 1.372.734 2.942 2.106 2.106a1.532 1.532 0 012.287.947c.379 1.561 2.6 1.561 2.978 0a1.533 1.533 0 012.287-.947c1.372.836 2.942-.734 2.106-2.106a1.533 1.533 0 01.947-2.287c1.561-.379 1.561-2.6 0-2.978a1.532 1.532 0 01-.947-2.287c.836-1.372-.734-2.942-2.106-2.106a1.532 1.532 0 01-2.287-.947zM10 13a3 3 0 100-6 3 3 0 000 6z"
                                    clip-rule="evenodd" />
                            </svg>
                            Triggered Job
                        </h3>
                        <div class="ml-6 space-y-1 text-sm">
                            <div>
                                <strong class="text-gray-700 dark:text-gray-300">Job Name:</strong>
                                <span class="ml-1 font-mono text-gray-600 dark:text-gray-400">{{ alert.jobInfo.jobName
                                }}</span>
                            </div>
                            <div>
                                <strong class="text-gray-700 dark:text-gray-300">Source:</strong>
                                <span class="ml-1 font-mono text-gray-600 dark:text-gray-400">{{
                                    alert.jobInfo.configMapName }}</span>
                            </div>
                            <div>
                                <strong class="text-gray-700 dark:text-gray-300">Image:</strong>
                                <span class="ml-1 font-mono text-gray-600 dark:text-gray-400">{{ alert.jobInfo.image
                                }}</span>
                            </div>
                        </div>
                    </div>
                </template>
                <template v-else>
                    <hr class="border-gray-200 dark:border-gray-700" />
                    <div>
                        <h3 class="flex items-center gap-2 text-sm font-semibold text-gray-500 dark:text-gray-400 mb-2">
                            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                                    d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                            </svg>
                            Remediation Status
                        </h3>
                        <div class="ml-6 text-sm text-gray-600 dark:text-gray-400 italic">
                            No matching Operarius found. Alert is displayed for monitoring purposes only.
                        </div>
                    </div>
                </template>

                <!-- Labels Section -->
                <hr class="border-gray-200 dark:border-gray-700" />
                <div>
                    <h3 class="flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white mb-2">
                        <svg class="w-4 h-4 text-gray-500" fill="currentColor" viewBox="0 0 20 20">
                            <path fill-rule="evenodd"
                                d="M17.707 9.293a1 1 0 010 1.414l-7 7a1 1 0 01-1.414 0l-7-7A.997.997 0 012 10V5a3 3 0 013-3h5c.256 0 .512.098.707.293l7 7zM5 6a1 1 0 100-2 1 1 0 000 2z"
                                clip-rule="evenodd" />
                        </svg>
                        Labels
                    </h3>
                    <template v-if="hasLabels">
                        <div class="ml-6 space-y-1 text-sm">
                            <div v-for="(value, key) in alert.alert.labels" :key="key">
                                <strong class="text-gray-700 dark:text-gray-300">{{ key }}:</strong>
                                <span class="ml-1 text-gray-600 dark:text-gray-400">{{ value }}</span>
                            </div>
                        </div>
                    </template>
                    <p v-else class="ml-6 text-sm text-gray-400 dark:text-gray-500">No labels found.</p>
                </div>

                <!-- Annotations Section -->
                <hr class="border-gray-200 dark:border-gray-700" />
                <div>
                    <h3 class="flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white mb-2">
                        <svg class="w-4 h-4 text-gray-500" fill="currentColor" viewBox="0 0 20 20">
                            <path fill-rule="evenodd"
                                d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z"
                                clip-rule="evenodd" />
                        </svg>
                        Annotations
                    </h3>
                    <template v-if="hasAnnotations">
                        <div class="ml-6 space-y-1 text-sm">
                            <div v-for="(value, key) in alert.alert.annotations" :key="key">
                                <strong class="text-gray-700 dark:text-gray-300">{{ key }}:</strong>
                                <span class="ml-1 text-gray-600 dark:text-gray-400">{{ value }}</span>
                            </div>
                        </div>
                    </template>
                    <p v-else class="ml-6 text-sm text-gray-400 dark:text-gray-500">No annotations found.</p>
                </div>
            </div>
        </div>
    </div>
</template>

<style scoped>
/* Minimal custom styles - most styling via Tailwind */
</style>
