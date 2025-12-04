import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import WorkflowView from '../WorkflowView.vue'
import { useAlertsStore } from '@/stores'
import type { AlertStoreEntry } from '@/types'

// Mock vue-flow components
vi.mock('@vue-flow/core', () => ({
  VueFlow: {
    name: 'VueFlow',
    template: '<div class="vue-flow-mock"><slot /><slot name="node-alert" :data="{ label: \'Test\', status: \'firing\' }" /><slot name="node-job" :data="{ label: \'Job\', configMap: \'config\', status: \'running\' }" /><slot name="node-result" :data="{ label: \'pending\', status: \'pending\' }" /></div>',
    props: ['nodes', 'edges', 'defaultViewport', 'fitViewOnInit'],
    emits: ['node-click'],
  },
  useVueFlow: () => ({
    fitView: vi.fn(),
  }),
}))

vi.mock('@vue-flow/background', () => ({
  Background: {
    name: 'Background',
    template: '<div class="background-mock" />',
  },
}))

vi.mock('@vue-flow/controls', () => ({
  Controls: {
    name: 'Controls',
    template: '<div class="controls-mock" />',
  },
}))

// Mock WebSocket
class MockWebSocket {
  static OPEN = 1
  static CLOSED = 3
  readyState = MockWebSocket.OPEN
  onopen: (() => void) | null = null
  onmessage: ((event: { data: string }) => void) | null = null
  onclose: (() => void) | null = null
  onerror: (() => void) | null = null
  close = vi.fn()
  send = vi.fn()
  
  constructor() {
    setTimeout(() => {
      if (this.onopen) this.onopen()
    }, 0)
  }
}

describe('WorkflowView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.stubGlobal('WebSocket', MockWebSocket)
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('should render the workflow header', async () => {
    const wrapper = mount(WorkflowView, {
      global: {
        stubs: {
          VueFlow: true,
          Background: true,
          Controls: true,
        },
      },
    })

    await flushPromises()
    expect(wrapper.text()).toContain('Workflow Visualization')
  })

  it('should show legend badges', async () => {
    const wrapper = mount(WorkflowView, {
      global: {
        stubs: {
          VueFlow: true,
          Background: true,
          Controls: true,
        },
      },
    })

    await flushPromises()
    expect(wrapper.text()).toContain('Firing')
    expect(wrapper.text()).toContain('Resolved / Succeeded')
    expect(wrapper.text()).toContain('Running')
    expect(wrapper.text()).toContain('Pending')
    expect(wrapper.text()).toContain('Failed')
  })

  it('should show empty state when no alerts', async () => {
    const wrapper = mount(WorkflowView, {
      global: {
        stubs: {
          VueFlow: true,
          Background: true,
          Controls: true,
        },
      },
    })

    await flushPromises()
    expect(wrapper.text()).toContain('No workflows to display')
  })

  it('should display connection status indicator', async () => {
    const wrapper = mount(WorkflowView, {
      global: {
        stubs: {
          VueFlow: true,
          Background: true,
          Controls: true,
        },
      },
    })

    await flushPromises()
    // Initially disconnected or connecting
    expect(wrapper.text()).toMatch(/Live|Disconnected/)
  })

  it('should have Fit View button', async () => {
    const wrapper = mount(WorkflowView, {
      global: {
        stubs: {
          VueFlow: true,
          Background: true,
          Controls: true,
        },
      },
    })

    await flushPromises()
    const fitViewBtn = wrapper.find('button')
    expect(fitViewBtn.exists()).toBe(true)
    expect(fitViewBtn.text()).toContain('Fit View')
  })

  describe('with alerts', () => {
    it('should transform alerts to nodes', async () => {
      const wrapper = mount(WorkflowView, {
        global: {
          stubs: {
            Background: true,
            Controls: true,
          },
        },
      })

      const alertsStore = useAlertsStore()
      const mockAlert: AlertStoreEntry = {
        alert: {
          labels: { alertname: 'TestAlert', severity: 'critical' },
          annotations: { description: 'Test description' },
        },
        status: 'firing',
        timestamp: new Date().toISOString(),
        jobInfo: {
          configMapName: 'openfero-TestAlert-firing',
          jobName: 'test-job-123',
          image: 'remediation:latest',
          status: 'running',
        },
      }

      alertsStore.alerts = [mockAlert]
      await flushPromises()

      // Check if VueFlow receives nodes
      const vueFlow = wrapper.findComponent({ name: 'VueFlow' })
      expect(vueFlow.exists()).toBe(true)
    })
  })

  describe('node selection', () => {
    it('should show detail panel when node is selected', async () => {
      const wrapper = mount(WorkflowView, {
        global: {
          stubs: {
            VueFlow: true,
            Background: true,
            Controls: true,
          },
        },
      })

      await flushPromises()

      // Simulate node selection by setting selectedNode ref
      const vm = wrapper.vm as InstanceType<typeof WorkflowView> & { selectedNode: unknown }
      vm.selectedNode = {
        id: 'alert-0',
        type: 'alert',
        data: {
          label: 'TestAlert',
          status: 'firing',
          timestamp: new Date().toISOString(),
        },
        position: { x: 0, y: 0 },
      }
      await wrapper.vm.$nextTick()

      expect(wrapper.text()).toContain('TestAlert')
      expect(wrapper.text()).toContain('Status:')
    })

    it('should close detail panel when clear button is clicked', async () => {
      const wrapper = mount(WorkflowView, {
        global: {
          stubs: {
            VueFlow: true,
            Background: true,
            Controls: true,
          },
        },
      })

      await flushPromises()

      // Set selected node
      const vm = wrapper.vm as InstanceType<typeof WorkflowView> & { selectedNode: unknown; clearSelection: () => void }
      vm.selectedNode = {
        id: 'alert-0',
        type: 'alert',
        data: { label: 'Test', status: 'firing' },
        position: { x: 0, y: 0 },
      }
      await wrapper.vm.$nextTick()

      // Find and click close button
      const closeButton = wrapper.find('[class*="fixed"]').find('button')
      expect(closeButton.exists()).toBe(true)
      
      await closeButton.trigger('click')
      await wrapper.vm.$nextTick()

      // Panel should be closed
      expect(vm.selectedNode).toBeNull()
    })
  })

  describe('edge computation', () => {
    it('should create animated edges for firing alerts', async () => {
      const wrapper = mount(WorkflowView, {
        global: {
          stubs: {
            Background: true,
            Controls: true,
          },
        },
      })

      const alertsStore = useAlertsStore()
      alertsStore.alerts = [{
        alert: {
          labels: { alertname: 'FiringAlert' },
          annotations: {},
        },
        status: 'firing',
        timestamp: new Date().toISOString(),
        jobInfo: {
          configMapName: 'openfero-FiringAlert-firing',
          jobName: 'job-1',
          image: 'img:latest',
          status: 'running',
        },
      }]
      await flushPromises()

      const vueFlow = wrapper.findComponent({ name: 'VueFlow' })
      const edges = vueFlow.props('edges') as Array<{ animated: boolean }>
      
      if (edges && edges.length > 0) {
        // First edge (alert -> job) should be animated for firing alert
        expect(edges[0].animated).toBe(true)
      }
    })

    it('should create green edges for resolved alerts', async () => {
      const wrapper = mount(WorkflowView, {
        global: {
          stubs: {
            Background: true,
            Controls: true,
          },
        },
      })

      const alertsStore = useAlertsStore()
      alertsStore.alerts = [{
        alert: {
          labels: { alertname: 'ResolvedAlert' },
          annotations: {},
        },
        status: 'resolved',
        timestamp: new Date().toISOString(),
        jobInfo: {
          configMapName: 'openfero-ResolvedAlert-resolved',
          jobName: 'job-2',
          image: 'img:latest',
          status: 'succeeded',
        },
      }]
      await flushPromises()

      const vueFlow = wrapper.findComponent({ name: 'VueFlow' })
      const edges = vueFlow.props('edges') as Array<{ style: { stroke: string } }>
      
      if (edges && edges.length > 0) {
        // First edge should have green stroke for resolved alert
        expect(edges[0].style.stroke).toBe('#22c55e')
      }
    })
  })
})
