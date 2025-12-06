<script setup lang="ts">
import { onMounted } from 'vue'
import { JobTable } from '@/components'
import { useJobsStore } from '@/stores'

const jobsStore = useJobsStore()

onMounted(() => {
    jobsStore.fetch()
})
</script>

<template>
    <div class="px-4 pt-4">
        <div class="flex justify-between items-center mb-4">
            <h4 class="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
                <svg class="w-5 h-5 text-gray-500" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd"
                        d="M11.49 3.17c-.38-1.56-2.6-1.56-2.98 0a1.532 1.532 0 01-2.286.948c-1.372-.836-2.942.734-2.106 2.106.54.886.061 2.042-.947 2.287-1.561.379-1.561 2.6 0 2.978a1.532 1.532 0 01.947 2.287c-.836 1.372.734 2.942 2.106 2.106a1.532 1.532 0 012.287.947c.379 1.561 2.6 1.561 2.978 0a1.533 1.533 0 012.287-.947c1.372.836 2.942-.734 2.106-2.106a1.533 1.533 0 01.947-2.287c1.561-.379 1.561-2.6 0-2.978a1.532 1.532 0 01-.947-2.287c.836-1.372-.734-2.942-2.106-2.106a1.532 1.532 0 01-2.287-.947zM10 13a3 3 0 100-6 3 3 0 000 6z"
                        clip-rule="evenodd" />
                </svg>
                Configured Remediation Rules
            </h4>
            <button class="btn btn-secondary btn-sm" :disabled="jobsStore.isLoading" @click="jobsStore.fetch()">
                <svg class="w-4 h-4 mr-1 inline" :class="{ 'animate-spin': jobsStore.isLoading }" fill="none"
                    stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                        d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                </svg>
                Refresh
            </button>
        </div>

        <!-- Backend unavailable state -->
        <div v-if="jobsStore.isBackendUnavailable" class="text-center py-12">
            <div class="max-w-sm mx-auto p-6 card">
                <svg class="w-12 h-12 mx-auto text-amber-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                        d="M3 15a4 4 0 004 4h9a5 5 0 10-.1-9.999 5.002 5.002 0 10-9.78 2.096A4.001 4.001 0 003 15z" />
                </svg>
                <h4 class="mt-4 text-lg font-semibold text-gray-900 dark:text-white">Backend Unavailable</h4>
                <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">{{ jobsStore.error }}</p>
                <p class="mt-1 text-xs text-gray-400 dark:text-gray-500">
                    Make sure the OpenFero backend is running and accessible.
                </p>
                <button class="btn btn-primary mt-4" @click="jobsStore.fetch()">
                    <svg class="w-4 h-4 mr-1 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                            d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                    </svg>
                    Try Again
                </button>
            </div>
        </div>

        <!-- Error state -->
        <div v-else-if="jobsStore.error"
            class="p-4 rounded-lg bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-400"
            role="alert">
            <div class="flex items-center gap-2">
                <svg class="w-5 h-5 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd"
                        d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z"
                        clip-rule="evenodd" />
                </svg>
                <span>{{ jobsStore.error }}</span>
                <button class="btn btn-danger btn-sm ml-auto" @click="jobsStore.fetch()">
                    Retry
                </button>
            </div>
        </div>

        <JobTable v-else :jobs="jobsStore.jobs" :loading="jobsStore.isLoading" />
    </div>
</template>

<style scoped>
/* Custom styles are handled by Tailwind utility classes */
</style>
