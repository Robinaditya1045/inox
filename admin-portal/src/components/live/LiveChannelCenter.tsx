import * as React from "react";
import { useLiveChannels } from "@/hooks/useLiveChannels";
import { LiveChannelForm } from "@/components/live/LiveChannelForm";
import { MetricCard } from "@/components/dashboard/MetricCard";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { LiveChannel, LiveChannelStatus } from "@/types/live";
import {
  AlertCircle,
  Loader2,
  RadioTower,
  RefreshCw,
  ShieldAlert,
  Trash2,
} from "lucide-react";

const STATUS_VARIANT: Record<
  LiveChannelStatus,
  "success" | "warning" | "destructive" | "outline"
> = {
  live: "success",
  resolving: "warning",
  degraded: "warning",
  idle: "outline",
  error: "destructive",
  disabled: "outline",
};

export function LiveChannelCenter() {
  const {
    channels,
    resolvers,
    isLoading,
    isDemoMode,
    error,
    refresh,
    createChannel,
    deleteChannel,
    testResolve,
  } = useLiveChannels();

  const liveCount = React.useMemo(
    // Demo fixtures must never be counted as live: an operator reading this number
    // would think a channel is serving when nothing is connected at all.
    () => (isDemoMode ? 0 : channels.filter((c) => c.status === "live").length),
    [channels, isDemoMode],
  );
  const erroredCount = React.useMemo(
    () =>
      channels.filter((c) => c.status === "error" || c.status === "degraded")
        .length,
    [channels],
  );

  return (
    <div className="space-y-6">
      <div className="flex flex-col justify-between gap-4 rounded-xl border border-zinc-800 bg-gradient-to-r from-zinc-900/80 to-zinc-950/80 p-4 backdrop-blur-md md:flex-row md:items-center">
        <div className="flex items-center space-x-3">
          <div className="rounded-lg border border-rose-500/20 bg-rose-500/10 p-2.5 text-rose-400">
            <RadioTower className="h-6 w-6" />
          </div>
          <div>
            <div className="flex items-center space-x-2">
              <h2 className="text-base font-bold tracking-tight text-white">
                Live Channels
              </h2>
              <Badge
                variant={isDemoMode ? "warning" : "success"}
                className="text-[10px]"
              >
                {isDemoMode ? "DEMO LIST" : "POSTGRES"}
              </Badge>
            </div>
            <p className="mt-0.5 font-mono text-xs text-zinc-400">
              Ingest a live stream from a manifest URL, a provider API, or a
              page, and serve it to rooms through the backend proxy.
            </p>
          </div>
        </div>

        <div className="flex items-center space-x-3">
          {error && (
            <span className="flex items-center rounded-md border border-amber-500/20 bg-amber-500/10 px-2.5 py-1 font-mono text-xs text-amber-400">
              <ShieldAlert className="mr-1 h-3.5 w-3.5" /> {error}
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

      <div className="grid gap-4 sm:grid-cols-3">
        <MetricCard
          title="Channels"
          value={String(channels.length)}
          icon={RadioTower}
          category="USE"
        />
        <MetricCard
          title="Serving now"
          value={String(liveCount)}
          icon={RadioTower}
          category="USE"
          iconColor="text-emerald-400"
        />
        <MetricCard
          title="Needing attention"
          value={String(erroredCount)}
          icon={AlertCircle}
          category="RED"
          iconColor={erroredCount > 0 ? "text-rose-400" : undefined}
        />
      </div>

      <LiveChannelForm
        resolvers={resolvers}
        onCreate={createChannel}
        onTest={testResolve}
      />

      <div className="overflow-hidden rounded-xl border border-zinc-800">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Channel</TableHead>
              <TableHead>Resolver</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Last resolved</TableHead>
              <TableHead>Upstream expires</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {channels.length === 0 && !isLoading && (
              <TableRow>
                <TableCell
                  colSpan={6}
                  className="py-10 text-center text-sm text-zinc-500"
                >
                  No live channels yet. Create one above — test the source
                  first.
                </TableCell>
              </TableRow>
            )}
            {channels.map((channel) => (
              <ChannelRow
                key={channel.id}
                channel={channel}
                onDelete={deleteChannel}
              />
            ))}
          </TableBody>
        </Table>
      </div>
    </div>
  );
}

function ChannelRow({
  channel,
  onDelete,
}: {
  channel: LiveChannel;
  onDelete: (id: string) => Promise<void>;
}) {
  const [isDeleting, setIsDeleting] = React.useState(false);

  const handleDelete = async () => {
    setIsDeleting(true);
    try {
      await onDelete(channel.id);
    } finally {
      setIsDeleting(false);
    }
  };

  return (
    <TableRow>
      <TableCell>
        <div className="font-medium text-zinc-100">
          {channel.title || channel.slug}
        </div>
        <div className="font-mono text-[11px] text-zinc-500">
          /{channel.slug}
        </div>
      </TableCell>
      <TableCell>
        <Badge variant="outline" className="text-[10px]">
          {channel.resolver}
        </Badge>
      </TableCell>
      <TableCell>
        <div className="space-y-1">
          <Badge
            variant={STATUS_VARIANT[channel.status] ?? "outline"}
            className="text-[10px]"
          >
            {channel.status.toUpperCase()}
          </Badge>
          {/* Resolver failures are what operators spend their time on, so the
              message is shown in full rather than truncated to a status word. */}
          {channel.last_error && (
            <p className="max-w-xs font-mono text-[11px] leading-snug text-rose-400/90">
              {channel.last_error}
            </p>
          )}
        </div>
      </TableCell>
      <TableCell className="font-mono text-xs text-zinc-400">
        {channel.last_resolved_at
          ? new Date(channel.last_resolved_at).toLocaleTimeString()
          : "—"}
      </TableCell>
      <TableCell className="font-mono text-xs text-zinc-400">
        {channel.upstream_expires_at
          ? new Date(channel.upstream_expires_at).toLocaleTimeString()
          : "not stated"}
      </TableCell>
      <TableCell className="text-right">
        <Button
          variant="ghost"
          size="sm"
          onClick={handleDelete}
          disabled={isDeleting}
        >
          {isDeleting ? (
            <Loader2 className="h-3.5 w-3.5 animate-spin" />
          ) : (
            <Trash2 className="h-3.5 w-3.5 text-rose-400" />
          )}
        </Button>
      </TableCell>
    </TableRow>
  );
}
