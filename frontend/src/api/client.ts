import { ApiResponseEnvelope, ApiError, HealthStatus } from '../types';

const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL || '/api').replace(/\/+$/, '');

export interface RequestOptions extends RequestInit {
  params?: Record<string, string | number | boolean | undefined>;
}

/**
 * Low-level HTTP client executing requests with consistent envelope parsing and error handling.
 */
export async function request<T>(endpoint: string, options: RequestOptions = {}): Promise<T> {
  const { params, headers, ...restOptions } = options;

  let url = `${API_BASE_URL}${endpoint.startsWith('/') ? endpoint : `/${endpoint}`}`;

  if (params) {
    const searchParams = new URLSearchParams();
    Object.entries(params).forEach(([key, val]) => {
      if (val !== undefined && val !== null) {
        searchParams.append(key, String(val));
      }
    });
    const queryString = searchParams.toString();
    if (queryString) {
      url += (url.includes('?') ? '&' : '?') + queryString;
    }
  }

  const defaultHeaders: HeadersInit = {
    'Accept': 'application/json',
  };

  if (restOptions.body && typeof restOptions.body === 'string') {
    defaultHeaders['Content-Type'] = 'application/json';
  }

  const mergedHeaders = {
    ...defaultHeaders,
    ...headers,
  };

  let response: Response;
  try {
    response = await fetch(url, {
      ...restOptions,
      headers: mergedHeaders,
    });
  } catch (err: unknown) {
    const errorMsg = err instanceof Error ? err.message : 'Network connection failed';
    throw new ApiError(`Network error: ${errorMsg}`, 'NETWORK_ERROR', 0);
  }

  let body: ApiResponseEnvelope<T> | null = null;
  const contentType = response.headers.get('content-type');
  if (contentType && contentType.includes('application/json')) {
    try {
      body = await response.json();
    } catch {
      throw new ApiError('Failed to parse JSON response from server', 'INVALID_JSON_RESPONSE', response.status);
    }
  }

  // Check HTTP status code
  if (!response.ok) {
    const errorCode = body?.error?.code || `HTTP_${response.status}`;
    const errorMessage = body?.error?.message || response.statusText || 'Request failed';
    const details = body?.error?.details;
    throw new ApiError(errorMessage, errorCode, response.status, details);
  }

  // Check backend application success flag
  if (body && typeof body === 'object') {
    if (body.success === false) {
      const code = body.error?.code || 'API_ERROR';
      const msg = body.error?.message || 'Operation failed';
      throw new ApiError(msg, code, response.status, body.error?.details);
    }
    // Return inner data if envelope used, otherwise raw body
    if ('data' in body && body.data !== undefined) {
      return body.data as T;
    }
    return body as unknown as T;
  }

  return {} as T;
}

export const apiClient = {
  get: <T>(endpoint: string, options?: RequestOptions) =>
    request<T>(endpoint, { ...options, method: 'GET' }),

  post: <T>(endpoint: string, data?: unknown, options?: RequestOptions) =>
    request<T>(endpoint, {
      ...options,
      method: 'POST',
      body: data ? JSON.stringify(data) : undefined,
    }),

  patch: <T>(endpoint: string, data?: unknown, options?: RequestOptions) =>
    request<T>(endpoint, {
      ...options,
      method: 'PATCH',
      body: data ? JSON.stringify(data) : undefined,
    }),

  delete: <T>(endpoint: string, options?: RequestOptions) =>
    request<T>(endpoint, { ...options, method: 'DELETE' }),

  getHealth: () =>
    request<HealthStatus>('/health', { method: 'GET' }),
};
