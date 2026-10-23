import * as React from "react";
import { ApiError, apiJson } from "@/lib/api";
import type { AdminRoom } from "@/types/rooms";

export interface UseAdminRoomsResult {
  rooms: AdminRoom[];
  isLoading: boolean;
  isDemoMode: boolean;
  error: string | null;
  refresh: () => Promise<void>;
  /** Deletes the rooms and returns the IDs the server actually deleted. */
  deleteRooms: (ids: string[]) => Promise<string[]>;
}

/** Who is in a room now changes quickly; the list is re-read this often while open. */
const POLL_MS = 15_000;

const hoursAgo = (h: number) =>
  new Date(Date.now() - h * 3_600_000).toISOString();

const MOCK_ROOMS: AdminRoom[] = [
  {
    id: "5f0e9a52-0d4b-4b8e-9a3e-1c0a7d2e4f61",
    name: "Friday movie night",
    owner_id: "u-robin",
    is_private: false,
    current_media_url: "https://cdn.inox.stream/hls/sintel/master.m3u8",
    created_at: hoursAgo(3),
    updated_at: hoursAgo(1),
    members: [
      {
        user_id: "u-robin",
        username: "robin",
        role: "owner",
        joined_at: hoursAgo(3),
      },
      {
        user_id: "u-alice",
        username: "alice",
        role: "moderator",
        joined_at: hoursAgo(2),
      },
      {
        user_id: "u-bob",
        username: "bob",
        role: "member",
        joined_at: hoursAgo(1),
      },
    ],
    activity: { viewers: 3, is_playing: true, kind: "vod" },
  },
  {
    id: "8a1d3c77-6e2f-4c19-b0d4-2e9f6a1b3c58",
    name: "Design team sync",
    owner_id: "u-maya",
    is_private: true,
    current_media_url: "",
    created_at: hoursAgo(26),
    updated_at: hoursAgo(20),
    members: [
      {
        user_id: "u-maya",
        username: "maya",
        role: "owner",
        joined_at: hoursAgo(26),
      },
      {
        user_id: "u-li",
        username: "li",
        role: "member",
        joined_at: hoursAgo(25),
      },
    ],
  },
  {
    id: "c3b2a190-7f8e-4d6c-9b5a-4e3d2c1b0a99",
    name: "Champions league live",
    owner_id: "u-sam",
    is_private: false,
    current_media_url: "",
    created_at: hoursAgo(50),
    updated_at: hoursAgo(2),
    members: [
      {
        user_id: "u-sam",
        username: "sam",
        role: "owner",
        joined_at: hoursAgo(50),
      },
    ],
    activity: { viewers: 1, is_playing: false, kind: "live" },
  },
  {
    id: "1e2f3a4b-5c6d-4e7f-8091-a2b3c4d5e6f7",
    name: "Old test room",
    owner_id: "u-test",
    is_private: true,
    current_media_url: "",
    created_at: hoursAgo(24 * 40),
    updated_at: hoursAgo(24 * 40),
    members: [
      {
        user_id: "u-test",
        username: "qa-bot",
        role: "owner",
        joined_at: hoursAgo(24 * 40),
      },
    ],
  },
];

/**
 * Every room on the server, public and private, for the admin room manager.
 *
 * Falls back to sample rooms only when the backend cannot be reached at all.
 * When it answers with an error (most often 403: the session is not listed in
 * ADMIN_EMAILS) that error is shown instead, since sample rooms would look
 * deletable when nothing could be.
 */
export function useAdminRooms(): UseAdminRoomsResult {
  const [rooms, setRooms] = React.useState<AdminRoom[]>([]);
  const [isLoading, setIsLoading] = React.useState(true);
  const [isDemoMode, setIsDemoMode] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const demoRef = React.useRef(false);

  const load = React.useCallback(async (silent: boolean) => {
    if (demoRef.current) return;
    if (!silent) setIsLoading(true);
    try {
      const data = await apiJson<AdminRoom[] | null>("/admin/rooms");
      setRooms(Array.isArray(data) ? data : []);
      setError(null);
    } catch (err) {
      if (silent) return;
      if (err instanceof ApiError) {
        setRooms([]);
        setError(
          err.status === 401 || err.status === 403
            ? "This session is not an administrator, so rooms can't be listed. Sign in with an address in ADMIN_EMAILS."
            : `Could not load rooms (${err.status}).`,
        );
      } else {
        demoRef.current = true;
        setIsDemoMode(true);
        setRooms(MOCK_ROOMS);
        setError("Backend unreachable. Showing sample rooms.");
      }
    } finally {
      if (!silent) setIsLoading(false);
    }
  }, []);

  React.useEffect(() => {
    load(false);
    const timer = setInterval(() => {
      if (document.visibilityState === "visible") load(true);
    }, POLL_MS);
    return () => clearInterval(timer);
  }, [load]);

  const refresh = React.useCallback(async () => {
    if (demoRef.current) {
      // Retry the real backend; stay on the samples if it is still away.
      demoRef.current = false;
      setIsDemoMode(false);
    }
    await load(false);
  }, [load]);

  const deleteRooms = React.useCallback(async (ids: string[]) => {
    if (ids.length === 0) return [];
    let deleted: string[];
    if (demoRef.current) {
      deleted = ids;
    } else {
      const res = await apiJson<{ deleted: string[] }>("/admin/rooms/delete", {
        method: "POST",
        body: JSON.stringify({ room_ids: ids }),
      });
      deleted = res.deleted ?? [];
    }
    // Rooms that were already gone are dropped too: the list was stale.
    const gone = new Set(ids);
    setRooms((prev) => prev.filter((r) => !gone.has(r.id)));
    return deleted;
  }, []);

  return { rooms, isLoading, isDemoMode, error, refresh, deleteRooms };
}
