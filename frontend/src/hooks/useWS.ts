import { useContext } from 'react';
import { WSContext, type WSContextValue } from '../contexts/ws.context';

export const useWS = (): WSContextValue => {
  const context = useContext(WSContext);
  if (!context) {
    throw new Error('useWS must be used within a WSProvider');
  }
  return context;
};
