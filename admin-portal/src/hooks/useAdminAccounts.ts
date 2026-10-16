import * as React from "react";
import { apiFetch } from "@/lib/api";
import type { AdminAccount } from "@/types/admin";

export interface UseAdminAccountsResult {
  accounts: AdminAccount[];
  isLoading: boolean;
  error: string | null;
  refresh: () => Promise<void>;
  /** Makes the user with this email or username an admin. */
  grant: (identifier: string) => Promise<AdminAccount>;
  revoke: (userId: string) => Promise<void>;
}

/**
 * Sends a request and returns its JSON body, throwing the backend's own reason on
 * failure: every error here is about the person an owner typed in, so a generic
 * message would leave them guessing.
 */
async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await apiFetch(path, init);
  const body = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(body?.error || `Request failed with status ${res.status}`);
  }
  return body as T;
}

/**
 * Owners and admins, for the Access tab. Unlike the other panels this has no demo
 * fixtures: a made-up list of who holds admin access is worse than an error.
 */
export function useAdminAccounts(): UseAdminAccountsResult {
  const [accounts, setAccounts] = React.useState<AdminAccount[]>([]);
  const [isLoading, setIsLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);

  // Every state update sits in a promise callback, so the mount effect below never
  // sets state synchronously; isLoading already starts out true.
  const load = React.useCallback(
    () =>
      request<AdminAccount[]>("/admin/admins")
        .then((data) => {
          setAccounts(Array.isArray(data) ? data : []);
          setError(null);
        })
        .catch((err) => {
          setError(
            err instanceof Error ? err.message : "Couldn't load admins.",
          );
        })
        .finally(() => setIsLoading(false)),
    [],
  );

  React.useEffect(() => {
    void load();
  }, [load]);

  const refresh = React.useCallback(async () => {
    setIsLoading(true);
    await load();
  }, [load]);

  const grant = React.useCallback(
    async (identifier: string): Promise<AdminAccount> => {
      const account = await request<AdminAccount>("/admin/admins", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ identifier }),
      });
      await refresh();
      return account;
    },
    [refresh],
  );

  const revoke = React.useCallback(
    async (userId: string) => {
      await request(`/admin/admins/${encodeURIComponent(userId)}`, {
        method: "DELETE",
      });
      await refresh();
    },
    [refresh],
  );

  return { accounts, isLoading, error, refresh, grant, revoke };
}
