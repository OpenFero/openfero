<script setup lang="ts">
import { onMounted } from 'vue'
import { RouterView } from 'vue-router'
import { NavBar } from '@/components'
import { useAppStore } from '@/stores'
import { useTheme } from '@/composables'

// Initialize theme
useTheme()

// Load build info on mount
const appStore = useAppStore()
onMounted(() => {
  appStore.fetchInfo()
})
</script>

<template>
  <NavBar />
  <main class="pt-16 min-h-screen bg-gray-50 dark:bg-[#121218]">
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
