<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink } from 'vue-router'
import BaseSpinner from '@/components/BaseSpinner.vue'
import { useDateTime } from '@/composables/useDateTime'
import type { AlertStoreEntry, JobInfo } from '@/types'

const {
  jobs,
  alerts = [],
  highlightedOperarius = '',
  loading = false,
} = defineProps<{
  jobs: JobInfo[]
  alerts?: AlertStoreEntry[]
  highlightedOperarius?: string
  loading?: boolean
}>()

const { formatDateTime } = useDateTime()
const expandedOperarii = ref<Set<string>>(new Set())

const alertsByOperarius = computed(() => {
  const grouped = new Map<string, AlertStoreEntry[]>()
  for (const entry of alerts) {
    const name = entry.jobInfo?.operariusName
    if (!name) continue
    const list = grouped.get(name) ?? []
    list.push(entry)
    grouped.set(name, list)
  }
  return grouped
})

function matchedAlertsFor(operariusName: string): AlertStoreEntry[] {
  return alertsByOperarius.value.get(operariusName) ?? []
}

function toggleExpanded(operariusName: string) {
  const next = new Set(expandedOperarii.value)
  if (next.has(operariusName)) {
    next.delete(operariusName)
  } else {
    next.add(operariusName)
  }
  expandedOperarii.value = next
}

const formatDate = (dateString?: string) => {
  if (!dateString) return '-'
  return formatDateTime(dateString, {
    includeMilliseconds: false,
    includeTimezone: false,
  })
}

const getStatusColor = (status?: string) => {
  if (!status) return 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300'

  const s = status.toLowerCase()
  if (s === 'successful' || s === 'succeeded')
    return 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300'
  if (s === 'failed') return 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-300'
  if (s === 'running' || s === 'executing')
    return 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300'
  if (s === 'pending')
    return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-300'
  if (s.includes('skipped'))
    return 'bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-300'

  return 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300'
}
</script>

<template>
  <div>
    <!-- Loading state -->
    <BaseSpinner
      v-if="loading"
      message="Loading jobs..."
    />

    <!-- Jobs table -->
    <template v-else>
      <div
        v-if="jobs.length > 0"
        class="overflow-x-auto rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm"
      >
        <table class="w-full text-sm text-left">
          <thead
            class="text-xs text-primary-900 dark:text-gray-300 uppercase bg-primary-100 dark:bg-gray-800 font-bold border-b border-primary-200 dark:border-gray-700"
          >
            <tr>
              <th
                scope="col"
                class="px-4 py-3"
              >
                Source / Operarius
              </th>
              <th
                scope="col"
                class="px-4 py-3"
              >
                Alert Selector
              </th>
              <th
                scope="col"
                class="px-4 py-3"
              >
                Matched Alerts
              </th>
              <th
                scope="col"
                class="px-4 py-3"
              >
                Container Image
              </th>
              <th
                scope="col"
                class="px-4 py-3"
              >
                Executions
              </th>
              <th
                scope="col"
                class="px-4 py-3"
              >
                Last Triggered / Job
              </th>
              <th
                scope="col"
                class="px-4 py-3"
              >
                Status
              </th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
            <template
              v-for="job in jobs"
              :key="job.operariusName"
            >
              <tr
                :class="[
                  'transition-colors',
                  highlightedOperarius && highlightedOperarius === job.operariusName
                    ? 'bg-primary-50/70 dark:bg-primary-900/20'
                    : 'bg-white dark:bg-gray-900 hover:bg-primary-50 dark:hover:bg-gray-800',
                ]"
              >
                <td class="px-4 py-3 font-medium text-gray-900 dark:text-white">
                  <div class="flex flex-wrap items-center gap-1.5">
                    <span>{{ job.operariusName }}</span>
                    <span
                      v-if="job.dryRun"
                      class="px-1.5 py-0.5 rounded text-[11px] font-medium bg-amber-100 text-amber-800 dark:bg-amber-900/60 dark:text-amber-200"
                    >Dry-Run</span>
                    <span
                      v-if="job.enabled === false"
                      class="px-1.5 py-0.5 rounded text-[11px] font-medium bg-gray-200 text-gray-700 dark:bg-gray-700 dark:text-gray-300"
                    >Disabled</span>
                    <span
                      v-if="job.priority"
                      class="px-1.5 py-0.5 rounded text-[11px] font-medium bg-blue-100 text-blue-800 dark:bg-blue-900/50 dark:text-blue-200"
                    >P{{ job.priority }}</span>
                  </div>
                  <div
                    v-if="job.namespace"
                    class="text-xs text-gray-400 mt-0.5"
                  >
                    ns: {{ job.namespace }}
                  </div>
                </td>
                <td class="px-4 py-3 text-gray-600 dark:text-gray-300">
                  <div class="flex flex-wrap items-center gap-1.5">
                    <span class="font-mono text-xs font-semibold text-gray-900 dark:text-white">{{
                      job.jobName }}</span>
                    <span
                      v-if="job.alertStatus"
                      class="px-1.5 py-0.5 rounded text-[11px] font-medium bg-red-100 text-red-800 dark:bg-red-900/50 dark:text-red-200"
                    >{{ job.alertStatus }}</span>
                  </div>
                  <div
                    v-if="job.alertLabels && Object.keys(job.alertLabels).length > 0"
                    class="flex flex-wrap gap-1 mt-1"
                  >
                    <span
                      v-for="(val, key) in job.alertLabels"
                      :key="key"
                      class="px-1.5 py-0.5 rounded text-[11px] font-mono bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-300"
                    >{{ key }}={{ val }}</span>
                  </div>
                </td>
                <td class="px-4 py-3">
                  <div class="flex items-center gap-2">
                    <RouterLink
                      :to="{ name: 'alerts', query: { q: job.operariusName } }"
                      class="inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium bg-primary-100 text-primary-800 dark:bg-primary-900/40 dark:text-primary-200 hover:underline"
                    >
                      {{ matchedAlertsFor(job.operariusName).length }}
                      {{ matchedAlertsFor(job.operariusName).length === 1 ? 'alert' : 'alerts' }}
                    </RouterLink>
                    <button
                      v-if="matchedAlertsFor(job.operariusName).length > 0"
                      type="button"
                      class="text-xs text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200"
                      @click="toggleExpanded(job.operariusName)"
                    >
                      {{ expandedOperarii.has(job.operariusName) ? 'Hide' : 'Preview' }}
                    </button>
                  </div>
                </td>
                <td class="px-4 py-3">
                  <code
                    class="text-xs font-mono bg-gray-100 dark:bg-gray-800 text-gray-700 dark:text-gray-300 px-2 py-0.5 rounded"
                  >{{ job.image }}</code>
                </td>
                <td class="px-4 py-3 text-gray-600 dark:text-gray-400">
                  {{ job.executionCount || 0 }}
                </td>
                <td class="px-4 py-3 text-gray-600 dark:text-gray-400">
                  <div
                    v-if="job.lastTriggeredTime || job.lastExecutionTime"
                    class="space-y-1 text-xs"
                  >
                    <div v-if="job.lastTriggeredTime">
                      <span class="text-gray-400">Triggered:</span>
                      {{ formatDate(job.lastTriggeredTime) }}
                    </div>
                    <div v-if="job.lastExecutionTime">
                      <span class="text-gray-400">Job:</span>
                      {{ formatDate(job.lastExecutionTime) }}
                      <div
                        v-if="job.lastExecutedJobName"
                        class="text-gray-400 mt-0.5 font-mono"
                      >
                        {{ job.lastExecutedJobName }}
                      </div>
                    </div>
                  </div>
                  <span
                    v-else
                    class="text-xs italic text-gray-400"
                  >Never triggered</span>
                </td>
                <td class="px-4 py-3">
                  <span
                    v-if="job.status"
                    :class="['px-2 py-0.5 rounded text-xs font-medium', getStatusColor(job.status)]"
                  >
                    {{ job.status }}
                  </span>
                  <span
                    v-else
                    class="text-gray-400"
                  >-</span>
                </td>
              </tr>
              <tr
                v-if="expandedOperarii.has(job.operariusName)"
                class="bg-gray-50/70 dark:bg-gray-800/40"
              >
                <td
                  colspan="7"
                  class="px-4 py-2.5 text-xs"
                >
                  <div class="font-medium text-gray-700 dark:text-gray-300 mb-1.5">
                    Recent alerts linked to {{ job.operariusName }} (showing up to 5):
                  </div>
                  <div class="space-y-1">
                    <div
                      v-for="entry in matchedAlertsFor(job.operariusName).slice(0, 5)"
                      :key="`${entry.alert.labels.alertname || 'unknown'}-${entry.timestamp}`"
                      class="flex flex-wrap items-center gap-2 text-gray-600 dark:text-gray-400"
                    >
                      <span class="font-mono text-gray-800 dark:text-gray-200">{{
                        entry.alert.labels.alertname }}</span>
                      <span class="px-1.5 py-0.5 rounded bg-gray-200 dark:bg-gray-700 text-[11px]">{{
                        entry.status }}</span>
                      <span v-if="entry.jobInfo?.status">→ {{ entry.jobInfo.status }}</span>
                      <span class="text-gray-400">{{ formatDate(entry.timestamp) }}</span>
                    </div>
                  </div>
                </td>
              </tr>
            </template>
          </tbody>
        </table>
      </div>

      <!-- Empty state -->
      <div
        v-else
        class="text-center py-12"
      >
        <svg
          class="w-16 h-16 mx-auto text-gray-300 dark:text-gray-600"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="1.5"
            d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4"
          />
        </svg>
        <p class="mt-4 text-lg text-gray-500 dark:text-gray-400">
          No jobs found.
        </p>
        <p class="mt-1 text-sm text-gray-400 dark:text-gray-500">
          Jobs are created when alerts trigger remediation Operarii.
        </p>
      </div>
    </template>
  </div>
</template>

<style scoped>
/* Custom styles are handled by Tailwind utility classes */
</style>
