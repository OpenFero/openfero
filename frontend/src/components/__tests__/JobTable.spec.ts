import { mount, RouterLinkStub } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import type { AlertStoreEntry, JobInfo } from '@/types'
import JobTable from '../JobTable.vue'

vi.mock('@/composables/useDateTime', () => ({
  useDateTime: () => ({
    formatDateTime: vi.fn((date: string) => `Formatted: ${date}`),
  }),
}))

describe('JobTable', () => {
  const sampleJobs: JobInfo[] = [
    {
      operariusName: 'pod-restart-dryrun',
      jobName: 'PodCrashLooping',
      namespace: 'openfero',
      image: 'ghcr.io/openfero/remediate:latest',
      alertStatus: 'firing',
      alertLabels: { severity: 'critical' },
      dryRun: true,
      priority: 10,
      executionCount: 0,
      status: 'Skipped: DryRun',
    },
  ]

  const sampleAlerts: AlertStoreEntry[] = [
    {
      timestamp: '2026-11-12T22:00:00Z',
      status: 'firing',
      alert: {
        labels: { alertname: 'PodCrashLooping', severity: 'critical' },
        annotations: {},
      },
      jobInfo: {
        operariusName: 'pod-restart-dryrun',
        jobName: 'N/A (DryRun)',
        image: 'ghcr.io/openfero/remediate:latest',
        status: 'Skipped: DryRun',
      },
    },
  ]

  it('displays selector metadata, dry-run badge, and linked alerts preview', async () => {
    const wrapper = mount(JobTable, {
      props: {
        jobs: sampleJobs,
        alerts: sampleAlerts,
      },
      global: {
        stubs: {
          RouterLink: RouterLinkStub,
        },
      },
    })

    expect(wrapper.text()).toContain('pod-restart-dryrun')
    expect(wrapper.text()).toContain('Dry-Run')
    expect(wrapper.text()).toContain('P10')
    expect(wrapper.text()).toContain('PodCrashLooping')
    expect(wrapper.text()).toContain('firing')
    expect(wrapper.text()).toContain('severity=critical')
    expect(wrapper.text()).toContain('1 alert')

    const link = wrapper.findComponent(RouterLinkStub)
    expect(link.props('to')).toEqual({
      name: 'alerts',
      query: { q: 'pod-restart-dryrun' },
    })

    await wrapper.find('button').trigger('click')
    expect(wrapper.text()).toContain('Recent alerts linked to pod-restart-dryrun')
  })
})
