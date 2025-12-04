import { apiGet } from './client'
import type { BuildInfo } from '@/types'

/**
 * Fetch application build info
 */
export async function fetchBuildInfo(): Promise<BuildInfo> {
  return apiGet<BuildInfo>('/api/about')
}
