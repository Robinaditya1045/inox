import { apiClient, APIError } from '../../api/client';
import type { User, LoginRequest, SignupRequest } from '../../types';
import { logger } from '../../utils/logger';

function mapSessionToUser(data: any): User {
  if ('user' in data && data.user) {
    return data.user;
  }
  return {
    id: data.user_id || data.id,
    username: data.username,
    email: data.email,
    created_at: data.created_at || new Date().toISOString(),
  };
}

export const authService = {
  async signup(data: SignupRequest): Promise<User> {
    logger.info('AuthService: Attempting signup', { username: data.username, email: data.email });
    const response = await apiClient.post<any>('/auth/signup', data);
    return mapSessionToUser(response);
  },

  async login(data: LoginRequest): Promise<User> {
    logger.info('AuthService: Attempting login', { email: data.email });
    const response = await apiClient.post<any>('/auth/login', data);
    return mapSessionToUser(response);
  },

  async logout(): Promise<void> {
    logger.info('AuthService: Attempting logout');
    await apiClient.post<void>('/auth/logout');
  },

  async getCurrentUser(): Promise<User | null> {
    try {
      logger.debug('AuthService: Fetching current user session');
      const response = await apiClient.get<any>('/users/me');
      if (response && (response.user_id || response.id || response.user)) {
        return mapSessionToUser(response);
      }
      return null;
    } catch (error) {
      if (error instanceof APIError && (error.status === 401 || error.status === 403)) {
        logger.debug('AuthService: No active session found (401/403)');
        return null;
      }
      logger.error('AuthService: Error fetching current user', { error });
      return null;
    }
  },
};
