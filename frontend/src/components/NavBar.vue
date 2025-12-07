<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useTheme } from '@/composables/useTheme'
import { useAppStore } from '@/stores/app'
import { useSocketStore } from '@/stores/socket'

defineProps<{
    showSearch?: boolean
}>()

const emit = defineEmits<{
    search: [query: string]
}>()

const route = useRoute()
const { theme, toggleTheme } = useTheme()
const appStore = useAppStore()
const socketStore = useSocketStore()

const isDark = computed(() => theme.value === 'dark')
const showAboutModal = ref(false)
const mobileMenuOpen = ref(false)

function handleToggleConnection() {
    console.log('NavBar: Toggling connection. Current state:', {
        isConnected: socketStore.isConnected,
        isPaused: socketStore.isPaused
    })
    socketStore.toggleConnection()
}

function handleSearch(event: Event) {
    const target = event.target as HTMLInputElement
    emit('search', target.value)
}

function openAboutModal() {
    showAboutModal.value = true
    if (!appStore.buildInfo) {
        appStore.fetchInfo()
    }
}

function closeAboutModal() {
    showAboutModal.value = false
}

// Close modal on escape key
function handleEscape(event: KeyboardEvent) {
    if (event.key === 'Escape' && showAboutModal.value) {
        closeAboutModal()
    }
}

onMounted(() => {
    document.addEventListener('keydown', handleEscape)
})
</script>

<template>
    <nav
        class="navbar fixed top-0 left-0 right-0 z-50 flex items-center justify-between px-4 py-3 bg-white dark:bg-dark-navbar border-b border-gray-200 dark:border-dark-border">
        <div class="flex items-center gap-4">
            <RouterLink
                class="navbar-brand text-xl font-bold text-gray-900 dark:text-white hover:text-gray-900 dark:hover:text-white"
                to="/">
                OpenFero <span class="text-amber-500 dark:text-amber-400">Alerts</span>
            </RouterLink>

            <div class="hidden lg:block w-px h-6 bg-gray-300 dark:bg-white/20"></div>

            <ul class="hidden lg:flex items-center gap-1">
                <li>
                    <RouterLink
                        class="nav-link px-3 py-2 rounded-md text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-white/10 transition-colors"
                        :class="{ 'text-gray-900 dark:text-white font-medium bg-gray-100 dark:bg-white/10': route.path === '/' }"
                        to="/">
                        Alerts
                    </RouterLink>
                </li>
                <li>
                    <RouterLink
                        class="nav-link px-3 py-2 rounded-md text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-white/10 transition-colors"
                        :class="{ 'text-gray-900 dark:text-white font-medium bg-gray-100 dark:bg-white/10': route.path === '/jobs' }"
                        to="/jobs">
                        Remediation Rules
                    </RouterLink>
                </li>
            </ul>
        </div>

        <div class="flex items-center gap-2">
            <!-- Connection Status -->
            <button
                class="hidden sm:flex items-center gap-2 mr-2 px-2 py-1 rounded-full bg-gray-100 dark:bg-white/10 hover:bg-gray-200 dark:hover:bg-white/20 transition-colors cursor-pointer"
                @click="handleToggleConnection()"
                :title="socketStore.isConnected ? 'Click to disconnect' : 'Click to connect'">
                <span :class="[
                    socketStore.isConnected ? 'bg-green-500 animate-pulse' : (socketStore.isPaused ? 'bg-gray-400' : 'bg-red-500'),
                    'w-2 h-2 rounded-full'
                ]"></span>
                <span class="text-xs font-medium"
                    :class="socketStore.isConnected ? 'text-green-700 dark:text-green-400' : (socketStore.isPaused ? 'text-gray-600 dark:text-gray-400' : 'text-red-700 dark:text-red-400')">
                    {{ socketStore.isConnected ? 'Live' : (socketStore.isPaused ? 'Paused' : 'Disconnected') }}
                </span>
            </button>

            <!-- Search input -->
            <div v-if="showSearch" class="relative hidden sm:block">
                <input type="search" placeholder="Search..."
                    class="w-48 focus:w-72 px-3 py-1.5 text-sm rounded-md bg-gray-100 dark:bg-white/10 text-gray-900 dark:text-white placeholder-gray-500 dark:placeholder-gray-400 border border-gray-300 dark:border-white/20 focus:border-primary-500 dark:focus:border-white/40 focus:outline-none focus:ring-2 focus:ring-primary-500/20 dark:focus:ring-white/20 transition-all duration-300"
                    @input="handleSearch" />
            </div>

            <!-- About button -->
            <button type="button"
                class="btn-icon p-2 text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-white/10 rounded-md transition-colors"
                aria-label="About OpenFero" @click="openAboutModal">
                <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                        d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
            </button>

            <!-- Theme toggle button -->
            <button
                class="btn-icon p-2 text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-white/10 rounded-md transition-colors"
                aria-label="Toggle theme" @click="toggleTheme">
                <svg v-if="isDark" class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd"
                        d="M10 2a1 1 0 011 1v1a1 1 0 11-2 0V3a1 1 0 011-1zm4 8a4 4 0 11-8 0 4 4 0 018 0zm-.464 4.95l.707.707a1 1 0 001.414-1.414l-.707-.707a1 1 0 00-1.414 1.414zm2.12-10.607a1 1 0 010 1.414l-.706.707a1 1 0 11-1.414-1.414l.707-.707a1 1 0 011.414 0zM17 11a1 1 0 100-2h-1a1 1 0 100 2h1zm-7 4a1 1 0 011 1v1a1 1 0 11-2 0v-1a1 1 0 011-1zM5.05 6.464A1 1 0 106.465 5.05l-.708-.707a1 1 0 00-1.414 1.414l.707.707zm1.414 8.486l-.707.707a1 1 0 01-1.414-1.414l.707-.707a1 1 0 011.414 1.414zM4 11a1 1 0 100-2H3a1 1 0 000 2h1z"
                        clip-rule="evenodd" />
                </svg>
                <svg v-else class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
                    <path d="M17.293 13.293A8 8 0 016.707 2.707a8.001 8.001 0 1010.586 10.586z" />
                </svg>
            </button>

            <!-- Mobile menu button -->
            <button
                class="lg:hidden p-2 text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-white/10 rounded-md transition-colors"
                aria-label="Toggle menu" @click="mobileMenuOpen = !mobileMenuOpen">
                <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16" />
                </svg>
            </button>
        </div>
    </nav>

    <!-- Mobile menu -->
    <div v-if="mobileMenuOpen"
        class="lg:hidden fixed top-14 left-0 right-0 z-40 bg-white dark:bg-dark-navbar border-b border-gray-200 dark:border-white/10 px-4 py-3">
        <ul class="flex flex-col gap-1">
            <li>
                <RouterLink
                    class="block px-3 py-2 rounded-md text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-white/10 transition-colors"
                    :class="{ 'text-gray-900 dark:text-white font-medium bg-gray-100 dark:bg-white/10': route.path === '/' }"
                    to="/" @click="mobileMenuOpen = false">
                    Alerts
                </RouterLink>
            </li>
            <li>
                <RouterLink
                    class="block px-3 py-2 rounded-md text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-white/10 transition-colors"
                    :class="{ 'text-gray-900 dark:text-white font-medium bg-gray-100 dark:bg-white/10': route.path === '/jobs' }"
                    to="/jobs" @click="mobileMenuOpen = false">
                    Jobs
                </RouterLink>
            </li>
            <li>
                <button
                    class="w-full text-left px-3 py-2 rounded-md text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-white/10 transition-colors flex items-center gap-2"
                    @click="handleToggleConnection()">
                    <span :class="[
                        socketStore.isConnected ? 'bg-green-500 animate-pulse' : (socketStore.isPaused ? 'bg-gray-400' : 'bg-red-500'),
                        'w-2 h-2 rounded-full'
                    ]"></span>
                    {{ socketStore.isConnected ? 'Live Connection' : (socketStore.isPaused ? 'Connection Paused' :
                        'Disconnected') }}
                </button>
            </li>
        </ul>
        <div v-if="showSearch" class="mt-3">
            <input type="search" placeholder="Search..."
                class="w-full px-3 py-2 text-sm rounded-md bg-gray-100 dark:bg-white/10 text-gray-900 dark:text-white placeholder-gray-500 dark:placeholder-gray-400 border border-gray-300 dark:border-white/20 focus:border-primary-500 dark:focus:border-white/40 focus:outline-none focus:ring-2 focus:ring-primary-500/20 dark:focus:ring-white/20"
                @input="handleSearch" />
        </div>
    </div>

    <!-- About Modal -->
    <Teleport to="body">
        <div v-if="showAboutModal" class="fixed inset-0 z-[1050] flex items-center justify-center bg-black/50"
            @click.self="closeAboutModal">
            <div class="w-full max-w-md mx-4 bg-white dark:bg-gray-800 rounded-lg shadow-xl">
                <div class="flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-gray-700">
                    <h5 class="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
                        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                                d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                        </svg>
                        About OpenFero
                    </h5>
                    <button type="button"
                        class="p-1 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 rounded transition-colors"
                        aria-label="Close" @click="closeAboutModal">
                        <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
                            <path fill-rule="evenodd"
                                d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z"
                                clip-rule="evenodd" />
                        </svg>
                    </button>
                </div>
                <div class="px-4 py-4">
                    <p class="text-lg text-gray-900 dark:text-white">Kubernetes-native self-healing framework</p>
                    <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">
                        OpenFero receives Alertmanager webhooks and executes remediation Jobs
                        defined in Operarius CRDs.
                    </p>
                    <hr class="my-4 border-gray-200 dark:border-gray-700" />
                    <div v-if="appStore.isLoading" class="text-center py-4">
                        <div
                            class="inline-block w-6 h-6 border-2 border-primary-500 border-t-transparent rounded-full animate-spin">
                        </div>
                    </div>
                    <dl v-else-if="appStore.buildInfo" class="grid grid-cols-3 gap-y-2 text-sm">
                        <dt class="font-medium text-gray-900 dark:text-gray-200">Version</dt>
                        <dd class="col-span-2 font-mono text-gray-600 dark:text-gray-400">{{ appStore.buildInfo.version
                        }}</dd>
                        <dt class="font-medium text-gray-900 dark:text-gray-200">Commit</dt>
                        <dd class="col-span-2 font-mono text-gray-600 dark:text-gray-400">{{ appStore.buildInfo.commit
                        }}</dd>
                        <dt class="font-medium text-gray-900 dark:text-gray-200">Build Date</dt>
                        <dd class="col-span-2 font-mono text-gray-600 dark:text-gray-400">{{
                            appStore.buildInfo.buildDate }}</dd>
                    </dl>
                </div>
                <div class="flex justify-end gap-2 px-4 py-3 border-t border-gray-200 dark:border-gray-700">
                    <a href="https://github.com/openfero/openfero" target="_blank" rel="noopener"
                        class="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-md hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors">
                        <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                            <path
                                d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z" />
                        </svg>
                        GitHub
                    </a>
                    <button type="button" class="btn btn-primary px-3 py-1.5 text-sm" @click="closeAboutModal">
                        Close
                    </button>
                </div>
            </div>
        </div>
    </Teleport>
</template>

<style scoped>
/* Navbar remains dark in both themes for consistency */
</style>
