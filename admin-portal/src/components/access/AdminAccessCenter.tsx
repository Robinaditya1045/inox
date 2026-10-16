import * as React from "react";
import {
  AlertTriangle,
  KeyRound,
  Loader2,
  RefreshCw,
  UserMinus,
  UserPlus,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useAdminAccounts } from "@/hooks/useAdminAccounts";
import type { AdminAccount } from "@/types/admin";

/**
 * Owner-only: who can use the admin portal. Owners come from ADMIN_EMAILS and are
 * listed for reference; admins are added and removed here.
 */
export function AdminAccessCenter() {
  const { accounts, isLoading, error, refresh, grant, revoke } =
    useAdminAccounts();
  const [identifier, setIdentifier] = React.useState("");
  const [isGranting, setIsGranting] = React.useState(false);
  const [feedback, setFeedback] = React.useState<{
    tone: "success" | "error";
    text: string;
  } | null>(null);

  const handleGrant = async (e: React.FormEvent) => {
    e.preventDefault();
    const value = identifier.trim();
    if (!value) return;
    setIsGranting(true);
    setFeedback(null);
    try {
      const account = await grant(value);
      setIdentifier("");
      setFeedback({
        tone: "success",
        text: `${account.username ?? account.email} is now an admin.`,
      });
    } catch (err) {
      setFeedback({
        tone: "error",
        text: err instanceof Error ? err.message : "Couldn't add that admin.",
      });
    } finally {
      setIsGranting(false);
    }
  };

  const ownerCount = accounts.filter((a) => a.role === "owner").length;
  const adminCount = accounts.length - ownerCount;

  return (
    <div className="space-y-6 animate-enter">
      <div className="flex flex-col justify-between gap-4 rounded-xl border border-zinc-800 bg-gradient-to-r from-zinc-900/80 to-zinc-950/80 p-4 backdrop-blur-md md:flex-row md:items-center">
        <div className="flex items-center space-x-3">
          <div className="rounded-lg border border-cyan-500/20 bg-cyan-500/10 p-2.5 text-cyan-400">
            <KeyRound className="h-6 w-6" />
          </div>
          <div>
            <h2 className="text-base font-bold tracking-tight text-white">
              Access
            </h2>
            <p className="mt-0.5 font-mono text-xs text-zinc-400">
              {ownerCount} {ownerCount === 1 ? "owner" : "owners"} ·{" "}
              {adminCount} {adminCount === 1 ? "admin" : "admins"}. Admins can
              use the whole portal but can't add or remove anyone.
            </p>
          </div>
        </div>

        <div className="flex items-center space-x-3">
          {error && (
            <span className="flex items-center rounded-md border border-rose-500/20 bg-rose-500/10 px-2.5 py-1 font-mono text-xs text-rose-400">
              <AlertTriangle className="mr-1 h-3.5 w-3.5" /> {error}
            </span>
          )}
          <Button
            variant="outline"
            onClick={() => void refresh()}
            disabled={isLoading}
          >
            {isLoading ? (
              <Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" />
            ) : (
              <RefreshCw className="mr-2 h-3.5 w-3.5" />
            )}
            Refresh
          </Button>
        </div>
      </div>

      <form
        onSubmit={handleGrant}
        className="space-y-3 rounded-xl border border-zinc-800 bg-zinc-900/40 p-4"
      >
        <div className="space-y-1.5">
          <Label htmlFor="admin-identifier">Add an admin</Label>
          <div className="flex flex-col gap-2 sm:flex-row">
            <Input
              id="admin-identifier"
              value={identifier}
              onChange={(e) => setIdentifier(e.target.value)}
              placeholder="Email or username"
              autoComplete="off"
              spellCheck={false}
              disabled={isGranting}
            />
            <Button
              type="submit"
              variant="emerald"
              disabled={isGranting || !identifier.trim()}
              className="shrink-0"
            >
              {isGranting ? (
                <Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" />
              ) : (
                <UserPlus className="mr-2 h-3.5 w-3.5" />
              )}
              Add admin
            </Button>
          </div>
          <p className="text-[11px] text-zinc-500">
            They need an Inox account first. Owners are managed in ADMIN_EMAILS,
            not here.
          </p>
        </div>
        <div aria-live="polite">
          {feedback && (
            <p
              className={`font-mono text-xs ${
                feedback.tone === "success"
                  ? "text-emerald-400"
                  : "text-rose-400"
              }`}
            >
              {feedback.text}
            </p>
          )}
        </div>
      </form>

      <div className="overflow-hidden rounded-xl border border-zinc-800">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>User</TableHead>
              <TableHead>Role</TableHead>
              <TableHead>Added</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {accounts.length === 0 && !isLoading && (
              <TableRow>
                <TableCell
                  colSpan={4}
                  className="py-10 text-center text-sm text-zinc-500"
                >
                  {error ? "Couldn't load the access list." : "No one yet."}
                </TableCell>
              </TableRow>
            )}
            {accounts.map((account) => (
              <AccountRow
                key={account.user_id ?? account.email}
                account={account}
                onRevoke={revoke}
              />
            ))}
          </TableBody>
        </Table>
      </div>
    </div>
  );
}

function AccountRow({
  account,
  onRevoke,
}: {
  account: AdminAccount;
  onRevoke: (userId: string) => Promise<void>;
}) {
  const [isRevoking, setIsRevoking] = React.useState(false);
  const [rowError, setRowError] = React.useState<string | null>(null);
  const isOwner = account.role === "owner";
  const name = account.username ?? account.email;

  const handleRevoke = async () => {
    if (!account.user_id) return;
    if (!window.confirm(`Remove admin access for "${name}"?`)) return;
    setIsRevoking(true);
    setRowError(null);
    try {
      await onRevoke(account.user_id);
    } catch (err) {
      setRowError(err instanceof Error ? err.message : "Couldn't remove.");
    } finally {
      setIsRevoking(false);
    }
  };

  return (
    <TableRow>
      <TableCell>
        <div className="font-medium text-zinc-100">
          {account.username ?? (
            // An owner address nobody has claimed yet. Sign-up doesn't verify
            // email, so whoever registers it first becomes an owner.
            <span className="flex items-center text-amber-400">
              <AlertTriangle className="mr-1 h-3.5 w-3.5" /> Not signed up yet
            </span>
          )}
        </div>
        <div className="font-mono text-[11px] text-zinc-500">
          {account.email}
        </div>
      </TableCell>
      <TableCell>
        <Badge variant={isOwner ? "cyan" : "outline"} className="text-[10px]">
          {isOwner ? "OWNER" : "ADMIN"}
        </Badge>
      </TableCell>
      <TableCell className="font-mono text-xs text-zinc-400">
        {isOwner
          ? "ADMIN_EMAILS"
          : [
              account.granted_by && `by ${account.granted_by}`,
              account.granted_at &&
                new Date(account.granted_at).toLocaleDateString(),
            ]
              .filter(Boolean)
              .join(" · ") || "—"}
      </TableCell>
      <TableCell className="text-right">
        {isOwner ? (
          <span className="font-mono text-[11px] text-zinc-600">
            Set in env
          </span>
        ) : (
          <div className="flex flex-col items-end gap-1">
            <Button
              variant="destructive-outline"
              size="sm"
              disabled={isRevoking}
              onClick={() => void handleRevoke()}
              className="h-7 px-2 text-[11px]"
            >
              {isRevoking ? (
                <Loader2 className="mr-1 h-3 w-3 animate-spin" />
              ) : (
                <UserMinus className="mr-1 h-3 w-3" />
              )}
              Remove
            </Button>
            {rowError && (
              <span className="font-mono text-[11px] text-rose-400">
                {rowError}
              </span>
            )}
          </div>
        )}
      </TableCell>
    </TableRow>
  );
}
