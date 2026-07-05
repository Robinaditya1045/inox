import { useContext } from 'react';
import { RTCContext, type RTCContextValue } from '../contexts/rtc.context';
export type { RTCContextValue as UseRTCReturn };

export const useRTC = (_roomId?: string): RTCContextValue => {
  const context = useContext(RTCContext);
  if (!context) {
    throw new Error('useRTC must be used within an RTCProvider');
  }
  return context;
};
