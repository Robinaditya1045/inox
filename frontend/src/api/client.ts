import { logger } from '../utils/logger';

function resolveBaseUrl(): string {
  const envUrl = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080/api/v1';
  if (window.location.hostname !== 'localhost' && window.location.hostname !== '127.0.0.1') {
    return envUrl.replace('localhost', window.location.hostname).replace('127.0.0.1', window.location.hostname);
  }
  return envUrl;
}

const BASE_URL = resolveBaseUrl();

export class APIError extends Error {
  status: number;
  data?: unknown;

  constructor(status: number, message: string, data?: unknown) {
    super(message);
    this.name = 'APIError';
    this.status = status;
    this.data = data;
  }
}

export interface RequestOptions extends RequestInit {
  params?: Record<string, string | number | boolean>;
}

async function request<T>(endpoint: string, options: RequestOptions = {}): Promise<T> {
  const { params, ...customConfig } = options;
  let url = `${BASE_URL}${endpoint}`;

  if (params) {
    const searchParams = new URLSearchParams();
    Object.entries(params).forEach(([key, value]) => {
      if (value !== undefined && value !== null) {
        searchParams.append(key, String(value));
      }
    });
    const queryString = searchParams.toString();
    if (queryString) {
      url += `?${queryString}`;
    }
  }

  const storedSessionId = localStorage.getItem('inox_session_id');
  const config: RequestInit = {
    ...customConfig,
    credentials: 'include', // Mandated by backend contracts for HttpOnly cookies
    headers: {
      'Content-Type': 'application/json',
      ...(storedSessionId ? { 'Authorization': `Bearer ${storedSessionId}`, 'X-Session-ID': storedSessionId } : {}),
      ...customConfig.headers,
    },
  };

  logger.debug('API Request Initiated', { method: config.method || 'GET', endpoint: url });

  try {
    const response = await fetch(url, config);
    
    // If status is 204 No Content, return null or empty object
    if (response.status === 204) {
      return {} as T;
    }

    const data = await response.json().catch(() => null);

    if (!response.ok) {
      logger.warn('API Error Response', {
        status: response.status,
        endpoint: url,
        error: data?.error || response.statusText,
      });

      throw new APIError(
        response.status,
        data?.error || `HTTP error! status: ${response.status}`,
        data
      );
    }

    logger.debug('API Request Successful', { status: response.status, endpoint: url });
    return data as T;
  } catch (error) {
    if (error instanceof APIError) {
      throw error;
    }
    logger.error('API Network/Execution Error', {
      endpoint: url,
      error: error instanceof Error ? error.message : String(error),
    });
    throw new APIError(500, 'Network request failed or server is unreachable');
  }
}

export const apiClient = {
  request,

  get<T>(endpoint: string, options?: RequestOptions): Promise<T> {
    return request<T>(endpoint, { ...options, method: 'GET' });
  },

  post<T>(endpoint: string, body?: unknown, options?: RequestOptions): Promise<T> {
    return request<T>(endpoint, {
      ...options,
      method: 'POST',
      body: body ? JSON.stringify(body) : undefined,
    });
  },

  put<T>(endpoint: string, body?: unknown, options?: RequestOptions): Promise<T> {
    return request<T>(endpoint, {
      ...options,
      method: 'PUT',
      body: body ? JSON.stringify(body) : undefined,
    });
  },

  delete<T>(endpoint: string, options?: RequestOptions): Promise<T> {
    return request<T>(endpoint, { ...options, method: 'DELETE' });
  },
};
