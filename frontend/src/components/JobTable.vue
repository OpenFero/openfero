<script setup lang="ts">
import type { JobInfo } from '@/types'

defineProps<{
  jobs: JobInfo[]
  loading?: boolean
}>()
</script>

<template>
    <div>
        <!-- Loading state -->
        <div v-if="loading" class="text-center py-12">
            <div
                class="inline-block w-8 h-8 border-4 border-primary-500 border-t-transparent rounded-full animate-spin">
            </div>
            <p class="mt-4 text-gray-500 dark:text-gray-400">Loading jobs...</p>
        </div>

        <!-- Jobs table -->
        <template v-else>
            <div v-if="jobs.length > 0"
                class="overflow-x-auto rounded-lg border border-gray-200 dark:border-gray-700 shadow-sm">
                <table class="w-full text-sm text-left">
                    <thead
                        class="text-xs text-primary-900 dark:text-gray-300 uppercase bg-primary-100 dark:bg-gray-800 font-bold border-b border-primary-200 dark:border-gray-700">
                        <tr>
                            <th scope="col" class="px-4 py-3">Source / Operarius</th>
                            <th scope="col" class="px-4 py-3">Target Alert / Job</th>
                            <th scope="col" class="px-4 py-3">Container Image</th>
                        </tr>
                    </thead>
                    <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
                        <tr v-for="job in jobs" :key="job.jobName"
                            class="bg-white dark:bg-gray-900 hover:bg-primary-50 dark:hover:bg-gray-800 transition-colors">
                            <td class="px-4 py-3 font-medium text-gray-900 dark:text-white">{{ job.configMapName }}</td>
                            <td class="px-4 py-3 text-gray-600 dark:text-gray-400">{{ job.jobName }}</td>
                            <td class="px-4 py-3">
                                <code
                                    class="text-sm font-mono bg-gray-100 dark:bg-gray-800 text-gray-700 dark:text-gray-300 px-2 py-0.5 rounded">{{ job.image }}</code>
                            </td>
                        </tr>
                    </tbody>
                </table>
            </div>

            <!-- Empty state -->
            <div v-else class="text-center py-12">
                <svg class="w-16 h-16 mx-auto text-gray-300 dark:text-gray-600" fill="none" stroke="currentColor"
                    viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5"
                        d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4" />
                </svg>
                <p class="mt-4 text-lg text-gray-500 dark:text-gray-400">No jobs found.</p>
                <p class="mt-1 text-sm text-gray-400 dark:text-gray-500">
                    Jobs are created when alerts trigger remediation ConfigMaps.
                </p>
            </div>
        </template>
    </div>
</template>

<style scoped>
/* Custom styles are handled by Tailwind utility classes */
</style>
