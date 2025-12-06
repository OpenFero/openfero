import { mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { AlertStoreEntry } from '@/types'
import AlertCard from '../AlertCard.vue'

// Mock useDateTime composable
vi.mock('@/composables/useDateTime', () => ({
  useDateTime: () => ({
    formatDateTime: vi.fn((date: string) => `Formatted: ${date}`),
    toISOString: vi.fn((date: string) => date),
  }),
}))

describe('AlertCard', () => {
  const mockAlert: AlertStoreEntry = {
    timestamp: '2025-12-04T14:30:00.123Z',
    status: 'firing',
    alert: {
      labels: {
        alertname: 'TestAlert',
        severity: 'critical',
        instance: 'server-1',
      },
      annotations: {
        summary: 'Test alert summary',
        description: 'Test alert description',
      },
    },
  }

  beforeEach(() => {
    // Mock navigator.language
    Object.defineProperty(global, 'navigator', {
      value: {
        language: 'en-US',
        languages: ['en-US'],
      },
      writable: true,
    })
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  describe('rendering', () => {
    it('should render the alert name', () => {
      const wrapper = mount(AlertCard, {
        props: {
          alert: mockAlert,
          index: 0,
          expanded: false,
        },
      })

      expect(wrapper.text()).toContain('TestAlert')
    })

    it('should render "Unknown Alert" when alertname is missing', () => {
      const alertWithoutName: AlertStoreEntry = {
        ...mockAlert,
        alert: {
          labels: {},
          annotations: {},
        },
      }

      const wrapper = mount(AlertCard, {
        props: {
          alert: alertWithoutName,
          index: 0,
          expanded: false,
        },
      })

      expect(wrapper.text()).toContain('Unknown Alert')
    })

    it('should show firing status color', () => {
      const wrapper = mount(AlertCard, {
        props: {
          alert: mockAlert,
          index: 0,
          expanded: false,
        },
      })

      const button = wrapper.find('button')
      expect(button.classes()).toContain('bg-red-600')
    })

    it('should show resolved status color', () => {
      const resolvedAlert: AlertStoreEntry = {
        ...mockAlert,
        status: 'resolved',
      }

      const wrapper = mount(AlertCard, {
        props: {
          alert: resolvedAlert,
          index: 0,
          expanded: false,
        },
      })

      const button = wrapper.find('button')
      expect(button.classes()).toContain('bg-green-600')
    })
  })

  describe('expansion', () => {
    it('should emit toggle event when clicked', async () => {
      const wrapper = mount(AlertCard, {
        props: {
          alert: mockAlert,
          index: 5,
          expanded: false,
        },
      })

      await wrapper.find('button').trigger('click')

      expect(wrapper.emitted('toggle')).toBeTruthy()
      expect(wrapper.emitted('toggle')?.[0]).toEqual([5])
    })

    it('should show details when expanded', () => {
      const wrapper = mount(AlertCard, {
        props: {
          alert: mockAlert,
          index: 0,
          expanded: true,
        },
      })

      expect(wrapper.text()).toContain('Metadata')
      expect(wrapper.text()).toContain('Labels')
      expect(wrapper.text()).toContain('Annotations')
    })

    it('should hide details when collapsed', () => {
      const wrapper = mount(AlertCard, {
        props: {
          alert: mockAlert,
          index: 0,
          expanded: false,
        },
      })

      // The accordion body should not be visible (v-show)
      const accordionBody = wrapper.find('.accordion-body')
      expect(accordionBody.isVisible()).toBe(false)
    })

    it('should have correct aria-expanded attribute', () => {
      const wrapper = mount(AlertCard, {
        props: {
          alert: mockAlert,
          index: 0,
          expanded: true,
        },
      })

      const button = wrapper.find('button')
      expect(button.attributes('aria-expanded')).toBe('true')
    })
  })

  describe('labels display', () => {
    it('should display all labels', () => {
      const wrapper = mount(AlertCard, {
        props: {
          alert: mockAlert,
          index: 0,
          expanded: true,
        },
      })

      expect(wrapper.text()).toContain('alertname')
      expect(wrapper.text()).toContain('TestAlert')
      expect(wrapper.text()).toContain('severity')
      expect(wrapper.text()).toContain('critical')
      expect(wrapper.text()).toContain('instance')
      expect(wrapper.text()).toContain('server-1')
    })
  })

  describe('annotations display', () => {
    it('should display all annotations', () => {
      const wrapper = mount(AlertCard, {
        props: {
          alert: mockAlert,
          index: 0,
          expanded: true,
        },
      })

      expect(wrapper.text()).toContain('summary')
      expect(wrapper.text()).toContain('Test alert summary')
      expect(wrapper.text()).toContain('description')
      expect(wrapper.text()).toContain('Test alert description')
    })
  })

  describe('timestamp display', () => {
    it('should display formatted timestamp when expanded', () => {
      const wrapper = mount(AlertCard, {
        props: {
          alert: mockAlert,
          index: 0,
          expanded: true,
        },
      })

      expect(wrapper.text()).toContain('Timestamp')
    })
  })

  describe('unique IDs', () => {
    it('should generate unique IDs based on index', () => {
      const wrapper = mount(AlertCard, {
        props: {
          alert: mockAlert,
          index: 42,
          expanded: false,
        },
      })

      const heading = wrapper.find('h2')
      expect(heading.attributes('id')).toBe('headingalert-42')
    })
  })
})
