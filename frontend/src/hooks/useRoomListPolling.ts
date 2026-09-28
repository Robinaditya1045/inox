import { useEffect } from "react";
import { useRoom } from "./useRoom";

const DEFAULT_INTERVAL_MS = 15_000;

/**
 * Keeps the room list — and with it each room's "now streaming" state — fresh
 * while the lobby is on screen. Refreshes on arrival, on an interval, and when
 * the tab comes back into view; pauses while the tab is hidden.
 */
export const useRoomListPolling = (intervalMs = DEFAULT_INTERVAL_MS) => {
  const { refreshRooms } = useRoom();

  useEffect(() => {
    const tick = () => {
      if (document.visibilityState === "visible") void refreshRooms();
    };
    // Deferred so arriving back from a room shows current activity straight away
    // without a synchronous state update inside the effect.
    const first = setTimeout(tick, 0);
    const interval = setInterval(tick, intervalMs);
    document.addEventListener("visibilitychange", tick);
    return () => {
      clearTimeout(first);
      clearInterval(interval);
      document.removeEventListener("visibilitychange", tick);
    };
  }, [refreshRooms, intervalMs]);
};
