import * as React from "react";
import { apiJson } from "@/lib/api";
import type { AdminMe, AdminRole } from "@/types/admin";

/**
 * The caller's admin portal role, or null while loading and whenever it can't be
 * confirmed: the backend is unreachable (demo mode) or the session has no admin
 * access. Owner-only controls stay hidden unless the backend says "owner", so an
 * unknown role never unlocks them.
 */
export function useAdminRole(): AdminRole | null {
  const [role, setRole] = React.useState<AdminRole | null>(null);

  React.useEffect(() => {
    let cancelled = false;
    apiJson<AdminMe>("/admin/me")
      .then((me) => {
        if (!cancelled) setRole(me.role);
      })
      .catch(() => {
        if (!cancelled) setRole(null);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return role;
}
