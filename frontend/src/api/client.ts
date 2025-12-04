/**
 * Base API client for OpenFero backend
 */

const BASE_URL = import.meta.env.VITE_API_URL || ''

/**
 * Custom API error with user-friendly messages
 */
export class ApiError extends Error {
  constructor(
    message: string,
    public readonly statusCode?: number,
    public readonly isNetworkError: boolean = false,
  ) {
    super(message)
    this.name = 'ApiError'
  }

  /**
   * Get a user-friendly error message
   */
  get userMessage(): string {
    if (this.isNetworkError) {
      return 'Unable to connect to OpenFero backend. Please check if the server is running.'
    }
    if (this.statusCode === 401) {
      return 'Authentication required. Please check your credentials.'
    }
    if (this.statusCode === 403) {
      return 'Access denied. You do not have permission to access this resource.'
    }
    if (this.statusCode === 404) {
      return 'Resource not found. The requested data does not exist.'
    }
    if (this.statusCode && this.statusCode >= 500) {
      return 'Server error. The OpenFero backend encountered an issue.'
    }
    return this.message
  }
}

/**
 * Parse response as JSON with proper error handling
 */
async function parseJsonResponse<T>(response: Response): Promise<T> {
  const contentType = response.headers.get('content-type')

  // Check if response is JSON
  if (!contentType || !contentType.includes('application/json')) {
    // If we got HTML (like 404 page or nginx error), provide helpful message
    const text = await response.text()
    if (text.startsWith('<!DOCTYPE') || text.startsWith('<html')) {
      throw new ApiError(
        'Backend returned HTML instead of JSON. This usually means the API endpoint is not available.',
        response.status,
      )
    }
    throw new ApiError(`Unexpected response format: ${contentType || 'unknown'}`, response.status)
  }

  try {
    return await response.json()
  } catch {
    throw new ApiError('Failed to parse server response as JSON', response.status)
  }
}

/**
 * Generic GET request with improved error handling
 */
export async function apiGet<T>(path: string): Promise<T> {
  let response: Response

  try {
    response = await fetch(`${BASE_URL}${path}`)
  } catch (error) {
    // Network error (server not running, CORS, etc.)
    if (error instanceof TypeError && error.message.includes('fetch')) {
      throw new ApiError('Network error: Unable to reach the server', undefined, true)
    }
    throw new ApiError(
      `Connection failed: ${error instanceof Error ? error.message : 'Unknown error'}`,
      undefined,
      true,
    )
  }

  if (!response.ok) {
    // Try to get error message from response body
    try {
      const errorData = await parseJsonResponse<{ error?: string; message?: string }>(response)
      throw new ApiError(
        errorData.error || errorData.message || `HTTP ${response.status}: ${response.statusText}`,
        response.status,
      )
    } catch (e) {
      if (e instanceof ApiError) throw e
      throw new ApiError(`HTTP ${response.status}: ${response.statusText}`, response.status)
    }
  }

  return parseJsonResponse<T>(response)
}

/**
 * Generic POST request with improved error handling
 */
export async function apiPost<T>(path: string, data: unknown): Promise<T> {
  let response: Response

  try {
    response = await fetch(`${BASE_URL}${path}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    })
  } catch (error) {
    // Network error (server not running, CORS, etc.)
    if (error instanceof TypeError && error.message.includes('fetch')) {
      throw new ApiError('Network error: Unable to reach the server', undefined, true)
    }
    throw new ApiError(
      `Connection failed: ${error instanceof Error ? error.message : 'Unknown error'}`,
      undefined,
      true,
    )
  }

  if (!response.ok) {
    // Try to get error message from response body
    try {
      const errorData = await parseJsonResponse<{ error?: string; message?: string }>(response)
      throw new ApiError(
        errorData.error || errorData.message || `HTTP ${response.status}: ${response.statusText}`,
        response.status,
      )
    } catch (e) {
      if (e instanceof ApiError) throw e
      throw new ApiError(`HTTP ${response.status}: ${response.statusText}`, response.status)
    }
  }

  return parseJsonResponse<T>(response)
}
