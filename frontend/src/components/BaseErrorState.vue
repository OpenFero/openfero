<script setup lang="ts">
const { error, isBackendUnavailable = false } = defineProps<{
  error: string
  isBackendUnavailable?: boolean
}>()

const emit = defineEmits<{
  retry: []
}>()
</script>

<template>
  <!-- Backend unavailable state -->
  <div
    v-if="isBackendUnavailable"
    class="text-center py-12"
  >
    <div class="max-w-sm mx-auto p-6 card">
      <svg
        class="w-12 h-12 mx-auto text-amber-500"
        fill="none"
        stroke="currentColor"
        viewBox="0 0 24 24"
        aria-hidden="true"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M3 15a4 4 0 004 4h9a5 5 0 10-.1-9.999 5.002 5.002 0 10-9.78 2.096A4.001 4.001 0 003 15z"
        />
      </svg>
      <h2 class="mt-4 text-lg font-semibold text-gray-900 dark:text-white">
        Backend Unavailable
      </h2>
      <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">
        {{ error }}
      </p>
      <p class="mt-1 text-xs text-gray-400 dark:text-gray-500">
        Make sure the OpenFero backend is running and accessible.
      </p>
      <button
        type="button"
        class="btn btn-primary mt-4"
        @click="emit('retry')"
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
            d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
          />
        </svg>
        Try Again
      </button>
    </div>
  </div>

  <!-- Error state -->
  <div
    v-else
    class="p-4 rounded-lg bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-400"
    role="alert"
  >
    <div class="flex items-center gap-2">
      <svg
        class="w-5 h-5 shrink-0"
        fill="currentColor"
        viewBox="0 0 20 20"
        aria-hidden="true"
      >
        <path
          fill-rule="evenodd"
          d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z"
          clip-rule="evenodd"
        />
      </svg>
      <span>{{ error }}</span>
      <button
        type="button"
        class="btn btn-danger btn-sm ml-auto"
        @click="emit('retry')"
      >
        Retry
      </button>
    </div>
  </div>
</template>
