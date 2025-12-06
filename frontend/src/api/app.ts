import type { BuildInfo } from '@/types'
import { apiGet } from './client'

/**
 * Fetch application build info
 */
export async function fetchBuildInfo(): Promise<BuildInfo> {
  return apiGet<BuildInfo>('/api/about')
}
