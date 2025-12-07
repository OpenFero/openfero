---
applyTo: "frontend/**"
---

# OpenFero Frontend Development Guidelines

## Tech Stack

- **Framework**: Vue.js 3 with Composition API
- **Language**: TypeScript (strict mode)
- **Build Tool**: Vite
- **State Management**: Pinia
- **Routing**: Vue Router 4
- **Workflow Visualization**: vue-flow
- **Styling**: Tailwind CSS
- **Testing**: Vitest + Vue Test Utils
- **Realtime**: Server-Sent Events (SSE)

## Project Structure

```text
frontend/
├── src/
│   ├── api/                 # API client functions
│   │   ├── alerts.ts        # Alert API calls
│   │   ├── jobs.ts          # Job API calls
│   │   └── client.ts        # Base fetch client
│   ├── components/          # Reusable UI components
│   │   ├── AlertCard.vue    # Single alert display
│   │   ├── JobNode.vue      # Job node for vue-flow
│   │   ├── WorkflowEdge.vue # Custom edge for workflow
│   │   ├── NavBar.vue       # Navigation bar
│   │   └── ThemeToggle.vue  # Dark/light mode toggle
│   ├── composables/         # Vue composables
│   │   ├── useSSE.ts        # Server-Sent Events
│   │   ├── useTheme.ts      # Theme management
│   │   └── useTimestamp.ts  # Timestamp formatting
│   ├── stores/              # Pinia stores
│   │   ├── alerts.ts        # Alert state
│   │   ├── jobs.ts          # Job state
│   │   └── workflow.ts      # Workflow graph state
│   ├── views/               # Page components
│   │   ├── AlertsView.vue   # Alerts list page
│   │   ├── JobsView.vue     # Jobs table page
│   │   ├── WorkflowView.vue # Workflow visualization
│   │   └── AboutView.vue    # About page
│   ├── types/               # TypeScript types
│   │   ├── alert.ts         # Alert interfaces
│   │   ├── job.ts           # Job interfaces
│   │   └── workflow.ts      # Workflow node/edge types
│   ├── App.vue              # Root component
│   ├── main.ts              # Application entry
│   └── router.ts            # Vue Router config
├── public/                  # Static assets
├── index.html               # HTML entry point
├── vite.config.ts           # Vite configuration
├── tsconfig.json            # TypeScript config
└── package.json             # Dependencies
```

## Coding Patterns

### Composition API with Script Setup

Always use `<script setup lang="ts">`:

```vue
<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { useAlertsStore } from "@/stores/alerts";

const alertsStore = useAlertsStore();
const searchQuery = ref("");

const filteredAlerts = computed(() =>
  alertsStore.alerts.filter((a) =>
    a.labels.alertname.includes(searchQuery.value)
  )
);

onMounted(() => {
  alertsStore.fetchAlerts();
});
</script>
```

### Pinia Store Pattern

```typescript
// stores/alerts.ts
import { defineStore } from "pinia";
import { ref, computed } from "vue";
import { fetchAlerts } from "@/api/alerts";
import type { AlertStoreEntry } from "@/types/alert";

export const useAlertsStore = defineStore("alerts", () => {
  const alerts = ref<AlertStoreEntry[]>([]);
  const isLoading = ref(false);
  const error = ref<string | null>(null);

  const firingAlerts = computed(() =>
    alerts.value.filter((a) => a.status === "firing")
  );

  async function fetch(query?: string) {
    isLoading.value = true;
    try {
      alerts.value = await fetchAlerts(query);
    } catch (e) {
      error.value = e instanceof Error ? e.message : "Unknown error";
    } finally {
      isLoading.value = false;
    }
  }

  function addAlert(alert: AlertStoreEntry) {
    alerts.value.unshift(alert);
  }

  return { alerts, isLoading, error, firingAlerts, fetch, addAlert };
});
```

### SSE Composable

```typescript
// composables/useSSE.ts
import { ref, onMounted, onUnmounted } from "vue";

export function useSSE(url: string) {
  const isConnected = ref(false);
  const lastEvent = ref<MessageEvent | null>(null);
  let eventSource: EventSource | null = null;

  function connect() {
    eventSource = new EventSource(url);

    eventSource.onopen = () => {
      isConnected.value = true;
    };

    eventSource.onerror = () => {
      isConnected.value = false;
      // Reconnect after 5 seconds
      setTimeout(connect, 5000);
    };

    eventSource.onmessage = (event) => {
      lastEvent.value = event;
    };
  }

  function addEventListener(type: string, handler: (data: unknown) => void) {
    eventSource?.addEventListener(type, (event) => {
      handler(JSON.parse((event as MessageEvent).data));
    });
  }

  onMounted(connect);

  onUnmounted(() => {
    eventSource?.close();
  });

  return { isConnected, lastEvent, addEventListener };
}
```

### vue-flow Workflow Pattern

```vue
<script setup lang="ts">
import { VueFlow, useVueFlow } from "@vue-flow/core";
import { Background } from "@vue-flow/background";
import { Controls } from "@vue-flow/controls";
import JobNode from "@/components/JobNode.vue";
import { useWorkflowStore } from "@/stores/workflow";

const workflowStore = useWorkflowStore();
const { fitView } = useVueFlow();

// Custom node types
const nodeTypes = {
  alert: AlertNode,
  job: JobNode,
  result: ResultNode,
};
</script>

<template>
  <VueFlow
    :nodes="workflowStore.nodes"
    :edges="workflowStore.edges"
    :node-types="nodeTypes"
    @nodes-initialized="fitView()"
  >
    <Background />
    <Controls />
  </VueFlow>
</template>
```

## TypeScript Types

### Alert Types

```typescript
// types/alert.ts
export interface Alert {
  labels: Record<string, string>;
  annotations: Record<string, string>;
  startsAt: string;
  endsAt: string;
  generatorURL?: string;
}

export interface JobInfo {
  operariusName: string;
  jobName: string;
  image: string;
  status?: "pending" | "running" | "succeeded" | "failed";
  startedAt?: string;
  completedAt?: string;
}

export interface AlertStoreEntry {
  alert: Alert;
  status: "firing" | "resolved";
  timestamp: string;
  jobInfo?: JobInfo;
}
```

## Styling Guidelines

- Use Bootstrap 5 utility classes where possible
- Custom CSS in `<style scoped>` blocks
- Support dark mode via `data-bs-theme="dark"` on `<html>`
- Use CSS custom properties for theme-aware colors

```vue
<style scoped>
.alert-card {
  border-left: 4px solid var(--bs-danger);
}

.alert-card.resolved {
  border-left-color: var(--bs-success);
}
</style>
```

## API Client Pattern

```typescript
// api/client.ts
const BASE_URL = import.meta.env.VITE_API_URL || "";

export async function apiGet<T>(path: string): Promise<T> {
  const response = await fetch(`${BASE_URL}${path}`);
  if (!response.ok) {
    throw new Error(`API error: ${response.status}`);
  }
  return response.json();
}

export async function apiPost<T>(path: string, data: unknown): Promise<T> {
  const response = await fetch(`${BASE_URL}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  });
  if (!response.ok) {
    throw new Error(`API error: ${response.status}`);
  }
  return response.json();
}
```

## Development Commands

```bash
# Development server with hot reload
cd frontend && npm run dev

# Type checking
npm run type-check

# Linting
npm run lint

# Unit tests
npm run test:unit

# Build for production
npm run build

# Preview production build
npm run preview
```

## Testing Patterns

```typescript
// components/__tests__/AlertCard.spec.ts
import { describe, it, expect } from "vitest";
import { mount } from "@vue/test-utils";
import AlertCard from "../AlertCard.vue";

describe("AlertCard", () => {
  it("renders alert name", () => {
    const wrapper = mount(AlertCard, {
      props: {
        alert: {
          labels: { alertname: "TestAlert" },
          annotations: {},
          startsAt: new Date().toISOString(),
          endsAt: "",
        },
        status: "firing",
      },
    });
    expect(wrapper.text()).toContain("TestAlert");
  });

  it("shows danger style for firing alerts", () => {
    const wrapper = mount(AlertCard, {
      props: { /* ... */ status: "firing" },
    });
    expect(wrapper.classes()).toContain("bg-danger");
  });
});
```

## Conventions

1. **File naming**: PascalCase for components (`AlertCard.vue`), camelCase for composables (`useSSE.ts`)
2. **Props**: Use `defineProps<T>()` with TypeScript interfaces
3. **Emits**: Use `defineEmits<T>()` with typed events
4. **Imports**: Use `@/` alias for src directory
5. **No emojis**: Keep code and comments professional, plain text only
