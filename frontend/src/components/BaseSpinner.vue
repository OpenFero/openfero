<script setup lang="ts">
const { message = '', size = 'md' } = defineProps<{
  message?: string
  size?: 'sm' | 'md'
}>()
</script>

<template>
  <div :class="size === 'sm' ? 'text-center py-4' : 'text-center py-12'">
    <div
      :class="[
        'inline-block border-primary-500 border-t-transparent rounded-full animate-spin',
        size === 'sm' ? 'w-6 h-6 border-2' : 'w-8 h-8 border-4',
      ]"
      role="status"
      :aria-label="message || 'Loading'"
    />
    <p
      v-if="message"
      class="mt-4 text-gray-500 dark:text-gray-400"
    >
      {{ message }}
    </p>
  </div>
</template>
