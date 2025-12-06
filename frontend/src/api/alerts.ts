import type { AlertStoreEntry, JobInfo } from '@/types'
import { apiGet } from './client'

/**
 * Fetch alerts from the API
 * @param query Optional search query
 */
export async function fetchAlerts(query?: string): Promise<AlertStoreEntry[]> {
  const path = query ? `/api/alerts?q=${encodeURIComponent(query)}` : '/api/alerts'
  return apiGet<AlertStoreEntry[]>(path)
}

/**
 * Fetch job definitions from ConfigMaps
 */
export async function fetchJobs(): Promise<JobInfo[]> {
  return apiGet<JobInfo[]>('/api/jobs')
}
