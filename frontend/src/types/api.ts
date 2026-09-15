/**
 * Standard API error response format from Go backend.
 */
export interface BackendErrorPayload {
  code: string;
  message: string;
  details?: unknown;
}

/**
 * Standard API envelope returned by all backend endpoints.
 */
export interface ApiResponseEnvelope<T> {
  success: boolean;
  data?: T;
  error?: BackendErrorPayload;
  message?: string;
}

/**
 * Structured client-side API error.
 */
export class ApiError extends Error {
  readonly code: string;
  readonly status: number;
  readonly details?: unknown;

  constructor(message: string, code: string = 'INTERNAL_ERROR', status: number = 500, details?: unknown) {
    super(message);
    this.name = 'ApiError';
    this.code = code;
    this.status = status;
    this.details = details;
  }
}

/**
 * Health check response payload.
 */
export interface HealthStatus {
  status: 'ok' | 'degraded';
  service: string;
  environment: string;
  database: 'connected' | 'unreachable' | 'unconfigured';
  timestamp: string;
}
