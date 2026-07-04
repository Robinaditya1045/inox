import { useAuth } from './useAuth';
import type { User } from '../types';

export const useCurrentUser = (): User | null => {
  const { user } = useAuth();
  return user;
};
