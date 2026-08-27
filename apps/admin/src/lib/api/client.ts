import { auth } from '../firebase'
import { signOut } from 'firebase/auth'

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080'

/**
 * Generic API error class
 */
export class ApiError extends Error {
  status: number
  data: unknown
  code?: string
  constructor(status: number, data: unknown, message: string, code?: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.data = data
    this.code = code
  }
}

/**
 * Specialized error types for better handling
 */
export class AuthError extends ApiError {
  constructor(data: unknown, message: string = 'Authentication failed') {
    super(401, data, message, 'AUTH_ERROR')
  }
}

export class NetworkError extends Error {
  constructor(message: string = 'Network request failed') {
    super(message)
    this.name = 'NetworkError'
  }
}

/**
 * Get the current auth token from localStorage
 */
export function getAuthToken(): string | null {
  return localStorage.getItem('admin_token')
}

/**
 * Set the auth token in localStorage
 */
export function setAuthToken(token: string): void {
  localStorage.setItem('admin_token', token)
}

/**
 * Clear the auth token from localStorage
 */
export function clearAuthToken(): void {
  localStorage.removeItem('admin_token')
}

/**
 * Extract error message from various response formats
 */
function extractErrorMessage(data: unknown, status: number): string {
  if (typeof data === 'string') {
    return data
  }
  if (data && typeof data === 'object') {
    const d = data as Record<string, unknown>
    if (typeof d.message === 'string') {
      return d.message
    }
    if (typeof d.error === 'string') {
      return d.error
    }
    if (typeof d.detail === 'string') {
      return d.detail
    }
    // Handle domain error codes
    if (typeof d.code === 'string') {
      // Map known domain error codes to user-friendly messages
      const errorMessages: Record<string, string> = {
        'CASE_NOT_APPEALABLE': 'This case cannot be appealed. It may have already been reviewed or removed.',
        'DUPLICATE_APPEAL': 'An appeal has already been submitted for this case.',
        'INVALID_STATE': 'This action cannot be performed in the current state.',
        'CASE_NOT_FOUND': 'The moderation case was not found.',
        'APPEAL_NOT_FOUND': 'The appeal was not found.',
        'WARNING_NOT_FOUND': 'The warning was not found.',
        'WARNING_ALREADY_ACTIVE': 'An active warning already exists for this user.',
        'UNAUTHORIZED': 'You do not have permission to perform this action.',
        'FORBIDDEN': 'Access denied. You do not have sufficient permissions.',
      }
      return errorMessages[d.code] || `An error occurred: ${d.code}`
    }
  }
  return `HTTP ${status}: An error occurred`
}

/**
 * Handle auth errors - redirect to login
 */
function handleAuthError(): never {
  clearAuthToken()
  window.location.href = '/login'
  throw new AuthError(null, 'Session expired. Please log in again.')
}

/**
 * Make an authenticated API request
 */
async function request<T>(
  endpoint: string,
  options: RequestInit = {}
): Promise<T> {
  const url = `${API_BASE_URL}${endpoint}`

  let response: Response
  let data: unknown

  try {
    const token = getAuthToken()

    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...(options.headers as Record<string, string>),
    }

    if (token) {
      headers['Authorization'] = `Bearer ${token}`
    }

    response = await fetch(url, {
      ...options,
      headers,
    })

    // Handle empty responses (e.g., 204 No Content)
    const contentType = response.headers.get('content-type')
    if (contentType && contentType.includes('application/json')) {
      data = await response.json()
    } else {
      data = null
    }
  } catch (error) {
    if (error instanceof TypeError) {
      // Network error (fetch failed)
      throw new NetworkError('Network request failed. Please check your connection.')
    }
    throw error
  }

  if (!response.ok) {
    // Handle 401 Unauthorized - redirect to login
    if (response.status === 401) {
      handleAuthError()
    }

    // Handle 403 Forbidden
    if (response.status === 403) {
      throw new ApiError(
        response.status,
        data,
        'Access denied. You do not have sufficient permissions for this action.',
        'FORBIDDEN'
      )
    }

    // Handle 404 Not Found
    if (response.status === 404) {
      throw new ApiError(
        response.status,
        data,
        'The requested resource was not found.',
        'NOT_FOUND'
      )
    }

    // Handle other errors
    const message = extractErrorMessage(data, response.status)
    const code = (data as Record<string, unknown>)?.code as string | undefined

    throw new ApiError(response.status, data, message, code)
  }

  return data as T
}

/**
 * API client with typed methods
 */
export const api = {
  get: <T>(endpoint: string) => request<T>(endpoint, { method: 'GET' }),

  post: <T>(endpoint: string, body: unknown) =>
    request<T>(endpoint, {
      method: 'POST',
      body: JSON.stringify(body),
    }),

  put: <T>(endpoint: string, body: unknown) =>
    request<T>(endpoint, {
      method: 'PUT',
      body: JSON.stringify(body),
    }),

  patch: <T>(endpoint: string, body: unknown) =>
    request<T>(endpoint, {
      method: 'PATCH',
      body: JSON.stringify(body),
    }),

  delete: <T>(endpoint: string) => request<T>(endpoint, { method: 'DELETE' }),
}

export async function logoutAdmin(): Promise<void> {
  await signOut(auth).catch(() => {})
  clearAuthToken()
}
