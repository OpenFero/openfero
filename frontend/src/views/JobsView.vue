<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { BaseErrorState, JobTable } from '@/components'
import { useAlertsStore, useJobsStore, useSocketStore } from '@/stores'
import type { JobInfo } from '@/types'

const route = useRoute()
const jobsStore = useJobsStore()
const alertsStore = useAlertsStore()
const socketStore = useSocketStore()
const showLegend = ref(false)

const selectedOperarius = computed(() =>
  typeof route.query.operarius === 'string' ? route.query.operarius : '',
)

const filteredJobs = computed(() => {
  if (!selectedOperarius.value) return jobsStore.jobs
  return jobsStore.jobs.filter((job) => job.operariusName === selectedOperarius.value)
})

const dryRunCount = computed(() => jobsStore.jobs.filter((job) => job.dryRun).length)

const matchedAlertCount = computed(
  () => alertsStore.alerts.filter((entry) => Boolean(entry.jobInfo?.operariusName)).length,
)

const unmatchedAlertGroups = computed(() => {
  const counts = new Map<string, number>()
  for (const entry of alertsStore.alerts) {
    if (entry.jobInfo?.operariusName) continue
    const name = entry.alert.labels.alertname || 'unknown'
    counts.set(name, (counts.get(name) ?? 0) + 1)
  }
  return Array.from(counts.entries()).map(([alertname, count]) => ({ alertname, count }))
})

const unmatchedAlertCount = computed(() =>
  unmatchedAlertGroups.value.reduce((sum, group) => sum + group.count, 0),
)

let removeListener: (() => void) | null = null

onMounted(() => {
  jobsStore.fetch()
  if (alertsStore.alerts.length === 0) {
    alertsStore.fetch()
  }
  removeListener = socketStore.addListener((message) => {
    if (message.type === 'operarius_update') {
      jobsStore.updateJob(message.data as JobInfo)
    } else if (message.type === 'alert') {
      alertsStore.fetch()
    }
  })
})

onUnmounted(() => {
  if (removeListener) removeListener()
})
</script>

<template>
  <div class="px-4 pt-4">
    <div class="flex justify-between items-center mb-4">
      <h1 class="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
        <svg
          class="w-5 h-5 text-gray-500"
          fill="currentColor"
          viewBox="0 0 20 20"
          aria-hidden="true"
        >
          <path
            fill-rule="evenodd"
            d="M11.49 3.17c-.38-1.56-2.6-1.56-2.98 0a1.532 1.532 0 01-2.286.948c-1.372-.836-2.942.734-2.106 2.106.54.886.061 2.042-.947 2.287-1.561.379-1.561 2.6 0 2.978a1.532 1.532 0 01.947 2.287c-.836 1.372.734 2.942 2.106 2.106a1.532 1.532 0 012.287.947c.379 1.561 2.6 1.561 2.978 0a1.533 1.533 0 012.287-.947c1.372.836 2.942-.734 2.106-2.106a1.533 1.533 0 01.947-2.287c1.561-.379 1.561-2.6 0-2.978a1.532 1.532 0 01-.947-2.287c.836-1.372-.734-2.942-2.106-2.106a1.532 1.532 0 01-2.287-.947zM10 13a3 3 0 100-6 3 3 0 000 6z"
            clip-rule="evenodd"
          />
        </svg>
        Configured Remediation Rules
      </h1>
      <div class="flex gap-2">
        <button
          type="button"
          class="btn btn-sm bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600"
          :aria-expanded="showLegend"
          @click="showLegend = !showLegend"
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
              d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
            />
          </svg>
          Legend
        </button>
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          :disabled="jobsStore.isLoading"
          @click="jobsStore.fetch()"
        >
          <svg
            class="w-4 h-4 mr-1 inline"
            :class="{ 'animate-spin': jobsStore.isLoading }"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
            aria-hidden="true"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
            />
          </svg>
          Refresh
        </button>
      </div>
    </div>

    <!-- Legend Modal/Panel -->
    <div
      v-if="showLegend"
      class="mb-4 p-4 bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm"
    >
      <h2 class="font-semibold text-gray-900 dark:text-white mb-2">
        Status Legend
      </h2>
      <div class="grid grid-cols-1 md:grid-cols-4 gap-4 text-sm">
        <div class="flex items-start gap-2">
          <span
            class="px-2 py-0.5 rounded text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300 mt-0.5"
          >Successful</span>
          <span class="text-gray-600 dark:text-gray-400">Last job creation was successful.</span>
        </div>
        <div class="flex items-start gap-2">
          <span
            class="px-2 py-0.5 rounded text-xs font-medium bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300 mt-0.5"
          >Running</span>
          <span class="text-gray-600 dark:text-gray-400">A remediation job is currently being
            processed.</span>
        </div>
        <div class="flex items-start gap-2">
          <span
            class="px-2 py-0.5 rounded text-xs font-medium bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-300 mt-0.5"
          >Pending</span>
          <span class="text-gray-600 dark:text-gray-400">Job is created but waiting to be scheduled.</span>
        </div>
        <div class="flex items-start gap-2">
          <span
            class="px-2 py-0.5 rounded text-xs font-medium bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-300 mt-0.5"
          >Failed</span>
          <span class="text-gray-600 dark:text-gray-400">Last job creation failed. Check logs for
            details.</span>
        </div>
      </div>
      <p class="mt-3 text-xs text-gray-500 dark:text-gray-500 italic">
        Tip: Hover over a status badge in the table to see detailed status messages.
      </p>
    </div>

    <!-- Coverage Summary & Operarius Filter -->
    <div
      v-if="!jobsStore.error"
      class="mb-4 space-y-3"
    >
      <div class="grid grid-cols-2 md:grid-cols-4 gap-3">
        <div class="p-3 rounded-lg bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700">
          <div class="text-xs text-gray-500 dark:text-gray-400">
            Operarii
          </div>
          <div class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">
            {{ jobsStore.jobs.length }}
          </div>
        </div>
        <div class="p-3 rounded-lg bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700">
          <div class="text-xs text-gray-500 dark:text-gray-400">
            Dry-Run Rules
          </div>
          <div class="mt-1 text-lg font-semibold text-amber-600 dark:text-amber-400">
            {{ dryRunCount }}
          </div>
        </div>
        <div class="p-3 rounded-lg bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700">
          <div class="text-xs text-gray-500 dark:text-gray-400">
            Matched Alerts
          </div>
          <div class="mt-1 text-lg font-semibold text-green-600 dark:text-green-400">
            {{ matchedAlertCount }}
          </div>
        </div>
        <div class="p-3 rounded-lg bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700">
          <div class="text-xs text-gray-500 dark:text-gray-400">
            Unmatched Alerts
          </div>
          <div class="mt-1 text-lg font-semibold text-gray-700 dark:text-gray-300">
            {{ unmatchedAlertCount }}
          </div>
        </div>
      </div>

      <div
        v-if="unmatchedAlertGroups.length > 0"
        class="p-3 rounded-lg bg-gray-50 dark:bg-gray-800/60 border border-gray-200 dark:border-gray-700 flex flex-wrap items-center gap-2 text-xs"
      >
        <span class="font-medium text-gray-600 dark:text-gray-300">
          Alerts without matching Operarius:
        </span>
        <RouterLink
          v-for="group in unmatchedAlertGroups"
          :key="group.alertname"
          :to="{ name: 'alerts', query: { q: group.alertname } }"
          class="inline-flex items-center gap-1 px-2 py-0.5 rounded bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:border-primary-500 hover:text-primary-600 dark:hover:text-primary-400"
        >
          <span class="font-mono">{{ group.alertname }}</span>
          <span class="text-gray-400">({{ group.count }})</span>
        </RouterLink>
      </div>

      <div
        v-if="selectedOperarius"
        class="flex items-center justify-between px-3 py-2 rounded-lg bg-primary-50 dark:bg-primary-900/30 border border-primary-200 dark:border-primary-800 text-sm"
      >
        <span class="text-primary-800 dark:text-primary-200">
          Focused on Operarius <code class="font-mono font-semibold">{{ selectedOperarius }}</code>
        </span>
        <RouterLink
          :to="{ name: 'jobs' }"
          class="text-xs font-medium text-primary-700 dark:text-primary-300 hover:underline"
        >
          Show all
        </RouterLink>
      </div>
    </div>

    <!-- Error / Backend unavailable state -->
    <BaseErrorState
      v-if="jobsStore.error"
      :error="jobsStore.error"
      :is-backend-unavailable="jobsStore.isBackendUnavailable"
      @retry="jobsStore.fetch()"
    />

    <JobTable
      v-else
      :jobs="filteredJobs"
      :alerts="alertsStore.alerts"
      :highlighted-operarius="selectedOperarius"
      :loading="jobsStore.isLoading"
    />
  </div>
</template>
