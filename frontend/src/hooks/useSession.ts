import { useAuth } from './useAuth';

export interface SessionState {
  isAuthenticated: boolean;
  isLoading: boolean;
  user: null | unknown;
}

export const useSession = () => {
  const { user, isLoading } = useAuth();
  return {
    isAuthenticated: !!user,
    isLoading,
    user,
  };
};
