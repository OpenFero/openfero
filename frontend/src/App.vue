<script setup lang="ts">
import { onMounted, useTemplateRef, watch } from 'vue'
import { RouterView, useRoute } from 'vue-router'
import { NavBar } from '@/components'
import { useTheme } from '@/composables'
import { useAppStore, useSocketStore } from '@/stores'

// Initialize theme
useTheme()

const route = useRoute()
const backToTop = useTemplateRef<HTMLElement>('backToTop')

watch(
  () => route.path,
  () => {
    backToTop.value?.focus()
  },
)

// Load build info and connect socket on mount
const appStore = useAppStore()
const socketStore = useSocketStore()

onMounted(() => {
  appStore.fetchInfo()
  socketStore.connect()
})
</script>

<template>
  <span
    ref="backToTop"
    tabindex="-1"
    class="sr-only"
  />
  <a
    href="#main"
    class="sr-only focus:not-sr-only focus:fixed focus:top-2 focus:left-2 focus:z-[1100] focus:px-4 focus:py-2 focus:bg-white focus:dark:bg-gray-800 focus:text-gray-900 focus:dark:text-white focus:rounded-md focus:shadow-lg focus:ring-2 focus:ring-primary-500"
  >
    Skip to main content
  </a>
  <NavBar />
  <main
    id="main"
    tabindex="-1"
    class="pt-16 min-h-screen bg-gray-50 dark:bg-[#121218] focus:outline-none"
  >
    <RouterView />
  </main>
</template>

<style>
/* Custom scrollbar */
::-webkit-scrollbar {
  width: 8px;
}

::-webkit-scrollbar-track {
  background-color: #f3f4f6;
}

.dark ::-webkit-scrollbar-track {
  background-color: #1f2937;
}

::-webkit-scrollbar-thumb {
  background-color: #d1d5db;
  border-radius: 4px;
}

.dark ::-webkit-scrollbar-thumb {
  background-color: #4b5563;
}

::-webkit-scrollbar-thumb:hover {
  background-color: #9ca3af;
}

.dark ::-webkit-scrollbar-thumb:hover {
  background-color: #6b7280;
}
</style>
