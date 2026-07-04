import React, { useState, useEffect, useCallback, type ReactNode } from 'react';
import { AuthContext } from '../contexts/auth.context';
import { authService } from '../services/auth/auth.service';
import type { User, LoginRequest, SignupRequest } from '../types';
import { logger } from '../utils/logger';
import { APIError } from '../api/client';

interface AuthProviderProps {
  children: ReactNode;
}

export const AuthProvider: React.FC<AuthProviderProps> = ({ children }) => {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  const clearError = useCallback(() => {
    setError(null);
  }, []);

  // Session recovery on mount
  useEffect(() => {
    let isMounted = true;
    const recoverSession = async () => {
      setIsLoading(true);
      try {
        const currentUser = await authService.getCurrentUser();
        if (isMounted) {
          setUser(currentUser);
        }
      } catch (err) {
        logger.error('AuthProvider: Failed to recover session', { err });
        if (isMounted) {
          setUser(null);
        }
      } finally {
        if (isMounted) {
          setIsLoading(false);
        }
      }
    };

    recoverSession();
    return () => {
      isMounted = false;
    };
  }, []);

  const login = useCallback(async (data: LoginRequest) => {
    setIsLoading(true);
    setError(null);
    try {
      const authenticatedUser = await authService.login(data);
      setUser(authenticatedUser);
      logger.info('AuthProvider: Login successful', { userId: authenticatedUser.id });
    } catch (err) {
      const msg = err instanceof APIError ? err.message : 'Login failed. Please check your credentials.';
      setError(msg);
      logger.warn('AuthProvider: Login failed', { error: msg });
      throw err;
    } finally {
      setIsLoading(false);
    }
  }, []);

  const signup = useCallback(async (data: SignupRequest) => {
    setIsLoading(true);
    setError(null);
    try {
      const newUser = await authService.signup(data);
      setUser(newUser);
      logger.info('AuthProvider: Signup successful', { userId: newUser.id });
    } catch (err) {
      const msg = err instanceof APIError ? err.message : 'Signup failed. Please try again.';
      setError(msg);
      logger.warn('AuthProvider: Signup failed', { error: msg });
      throw err;
    } finally {
      setIsLoading(false);
    }
  }, []);

  const logout = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      await authService.logout();
      setUser(null);
      logger.info('AuthProvider: Logout successful');
    } catch (err) {
      logger.error('AuthProvider: Logout error', { err });
      // Even if server logout fails, clear local user state
      setUser(null);
    } finally {
      setIsLoading(false);
    }
  }, []);

  const value = {
    user,
    isLoading,
    error,
    login,
    signup,
    logout,
    clearError,
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
};
