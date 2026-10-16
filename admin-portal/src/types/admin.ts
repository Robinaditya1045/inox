/**
 * Admin portal access. Owners are the addresses in the backend's ADMIN_EMAILS;
 * admins are users an owner added from the Access tab. Both can use the whole
 * portal, but only owners can add or remove admins.
 */
export type AdminRole = "owner" | "admin";

/** GET /admin/me: who the caller is and which role they hold. */
export interface AdminMe {
  user_id: string;
  username: string;
  email: string;
  role: AdminRole;
}

/** One person with admin portal access, as listed to owners. */
export interface AdminAccount {
  /** Missing for an owner address nobody has signed up with yet. */
  user_id?: string;
  username?: string;
  email: string;
  role: AdminRole;
  /** Username of the owner who added an admin. */
  granted_by?: string;
  granted_at?: string;
}
