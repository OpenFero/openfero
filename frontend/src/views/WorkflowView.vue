<script setup lang="ts">
import { onMounted, onUnmounted, computed } from 'vue'
import { VueFlow, useVueFlow } from '@vue-flow/core'
import { Background } from '@vue-flow/background'
import { Controls } from '@vue-flow/controls'
import type { Node, Edge } from '@vue-flow/core'
import { useAlertsStore } from '@/stores'
import { useSSE } from '@/composables'

const alertsStore = useAlertsStore()

// SSE for real-time updates
const { connect, disconnect, isConnected } = useSSE('/api/events', {
    onMessage: (event) => {
        if (event.type === 'alert' || event.type === 'job_status') {
            alertsStore.fetch()
        }
    },
})

const { fitView } = useVueFlow()

// Transform alerts to workflow nodes and edges
const nodes = computed<Node[]>(() => {
    const result: Node[] = []
    let yOffset = 50

    alertsStore.alerts.forEach((entry, index) => {
        const alertName = entry.alert.labels.alertname || 'Unknown'

        // Alert node
        result.push({
            id: `alert-${index}`,
            type: 'alert',
            position: { x: 50, y: yOffset },
            data: {
                label: alertName,
                status: entry.status,
                timestamp: entry.timestamp,
            },
        })

        // Job node (if job was triggered)
        if (entry.jobInfo) {
            result.push({
                id: `job-${index}`,
                type: 'job',
                position: { x: 350, y: yOffset },
                data: {
                    label: entry.jobInfo.jobName,
                    configMap: entry.jobInfo.configMapName,
                    image: entry.jobInfo.image,
                    status: entry.jobInfo.status,
                },
            })

            // Result node
            result.push({
                id: `result-${index}`,
                type: 'result',
                position: { x: 650, y: yOffset },
                data: {
                    label: entry.jobInfo.status || 'pending',
                    status: entry.jobInfo.status,
                },
            })
        }

        yOffset += 120
    })

    return result
})

const edges = computed<Edge[]>(() => {
    const result: Edge[] = []

    alertsStore.alerts.forEach((entry, index) => {
        if (entry.jobInfo) {
            // Alert -> Job edge
            result.push({
                id: `edge-alert-job-${index}`,
                source: `alert-${index}`,
                target: `job-${index}`,
                animated: entry.status === 'firing',
                style: { stroke: entry.status === 'firing' ? '#dc3545' : '#198754' },
            })

            // Job -> Result edge
            result.push({
                id: `edge-job-result-${index}`,
                source: `job-${index}`,
                target: `result-${index}`,
                animated: entry.jobInfo.status === 'running',
                style: {
                    stroke:
                        entry.jobInfo.status === 'succeeded'
                            ? '#198754'
                            : entry.jobInfo.status === 'failed'
                                ? '#dc3545'
                                : '#6c757d',
                },
            })
        }
    })

    return result
})

onMounted(() => {
    alertsStore.fetch()
    connect()
    setTimeout(() => fitView(), 100)
})

onUnmounted(() => {
    disconnect()
})
</script>

<template>
    <div class="workflow-view">
        <div class="workflow-header d-flex justify-content-between align-items-center mb-3">
            <h4 class="mb-0">
                <i class="bi bi-diagram-3-fill me-2"></i>Workflow Visualization
            </h4>
            <div class="d-flex align-items-center gap-3">
                <span class="text-muted">
                    <i class="bi" :class="isConnected ? 'bi-broadcast text-success' : 'bi-broadcast text-danger'"></i>
                    {{ isConnected ? 'Live' : 'Disconnected' }}
                </span>
                <button class="btn btn-outline-primary btn-sm" @click="fitView()">
                    <i class="bi bi-arrows-fullscreen"></i> Fit View
                </button>
            </div>
        </div>

        <!-- Legend -->
        <div class="workflow-legend mb-3">
            <span class="badge bg-danger me-2">
                <i class="bi bi-exclamation-triangle-fill"></i> Firing
            </span>
            <span class="badge bg-success me-2">
                <i class="bi bi-check-circle-fill"></i> Resolved / Succeeded
            </span>
            <span class="badge bg-primary me-2">
                <i class="bi bi-play-fill"></i> Running
            </span>
            <span class="badge bg-warning text-dark me-2">
                <i class="bi bi-hourglass-split"></i> Pending
            </span>
            <span class="badge bg-secondary">
                <i class="bi bi-x-circle-fill"></i> Failed
            </span>
        </div>

        <!-- Vue Flow Canvas -->
        <div class="workflow-canvas">
            <VueFlow :nodes="nodes" :edges="edges" :default-viewport="{ zoom: 1, x: 0, y: 0 }" fit-view-on-init>
                <Background />
                <Controls />

                <!-- Custom Alert Node -->
                <template #node-alert="{ data }">
                    <div class="workflow-node alert-node" :class="{
                        'status-firing': data.status === 'firing',
                        'status-resolved': data.status === 'resolved',
                    }">
                        <div class="node-icon">
                            <i class="bi bi-exclamation-triangle-fill"></i>
                        </div>
                        <div class="node-content">
                            <div class="node-label">{{ data.label }}</div>
                            <div class="node-status">{{ data.status }}</div>
                        </div>
                    </div>
                </template>

                <!-- Custom Job Node -->
                <template #node-job="{ data }">
                    <div class="workflow-node job-node" :class="{
                        'status-running': data.status === 'running',
                        'status-succeeded': data.status === 'succeeded',
                        'status-failed': data.status === 'failed',
                    }">
                        <div class="node-icon">
                            <i class="bi bi-gear-fill"></i>
                        </div>
                        <div class="node-content">
                            <div class="node-label">{{ data.label }}</div>
                            <div class="node-meta">{{ data.configMap }}</div>
                        </div>
                    </div>
                </template>

                <!-- Custom Result Node -->
                <template #node-result="{ data }">
                    <div class="workflow-node result-node" :class="{
                        'status-succeeded': data.status === 'succeeded',
                        'status-failed': data.status === 'failed',
                        'status-pending': !data.status || data.status === 'pending',
                        'status-running': data.status === 'running',
                    }">
                        <div class="node-icon">
                            <i class="bi" :class="{
                                'bi-check-circle-fill': data.status === 'succeeded',
                                'bi-x-circle-fill': data.status === 'failed',
                                'bi-hourglass-split': data.status === 'pending' || !data.status,
                                'bi-play-fill': data.status === 'running',
                            }"></i>
                        </div>
                        <div class="node-content">
                            <div class="node-label">{{ data.label }}</div>
                        </div>
                    </div>
                </template>
            </VueFlow>

            <!-- Empty state -->
            <div v-if="nodes.length === 0" class="workflow-empty">
                <i class="bi bi-diagram-3 fs-1 text-muted"></i>
                <p class="fs-4 text-muted mt-3">No workflows to display.</p>
                <p class="text-muted">
                    Workflows will appear here when alerts trigger remediation jobs.
                </p>
            </div>
        </div>
    </div>
</template>

<style scoped>
.workflow-view {
    padding: 1rem;
    height: calc(100vh - 90px);
    display: flex;
    flex-direction: column;
}

.workflow-canvas {
    flex: 1;
    border: 1px solid var(--bs-border-color);
    border-radius: 0.5rem;
    background-color: var(--bs-tertiary-bg);
    position: relative;
    min-height: 400px;
}

.workflow-empty {
    position: absolute;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    text-align: center;
}

/* Custom node styles */
.workflow-node {
    display: flex;
    align-items: center;
    padding: 0.75rem 1rem;
    border-radius: 0.5rem;
    background-color: var(--bs-body-bg);
    border: 2px solid var(--bs-border-color);
    min-width: 180px;
    box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.node-icon {
    font-size: 1.5rem;
    margin-right: 0.75rem;
}

.node-content {
    flex: 1;
}

.node-label {
    font-weight: 600;
    font-size: 0.875rem;
}

.node-status,
.node-meta {
    font-size: 0.75rem;
    color: var(--bs-secondary-color);
}

/* Alert node status */
.alert-node.status-firing {
    border-color: #dc3545;
    background-color: rgba(220, 53, 69, 0.1);
}

.alert-node.status-firing .node-icon {
    color: #dc3545;
}

.alert-node.status-resolved {
    border-color: #198754;
    background-color: rgba(25, 135, 84, 0.1);
}

.alert-node.status-resolved .node-icon {
    color: #198754;
}

/* Job node status */
.job-node .node-icon {
    color: #0d6efd;
}

.job-node.status-running {
    border-color: #0d6efd;
    background-color: rgba(13, 110, 253, 0.1);
}

.job-node.status-succeeded {
    border-color: #198754;
}

.job-node.status-failed {
    border-color: #dc3545;
}

/* Result node status */
.result-node.status-succeeded {
    border-color: #198754;
    background-color: rgba(25, 135, 84, 0.1);
}

.result-node.status-succeeded .node-icon {
    color: #198754;
}

.result-node.status-failed {
    border-color: #dc3545;
    background-color: rgba(220, 53, 69, 0.1);
}

.result-node.status-failed .node-icon {
    color: #dc3545;
}

.result-node.status-pending {
    border-color: #ffc107;
    background-color: rgba(255, 193, 7, 0.1);
}

.result-node.status-pending .node-icon {
    color: #ffc107;
}

.result-node.status-running {
    border-color: #0d6efd;
    background-color: rgba(13, 110, 253, 0.1);
}

.result-node.status-running .node-icon {
    color: #0d6efd;
}
</style>

<!-- Import vue-flow styles -->
<style>
@import '@vue-flow/core/dist/style.css';
@import '@vue-flow/core/dist/theme-default.css';
@import '@vue-flow/controls/dist/style.css';
</style>
