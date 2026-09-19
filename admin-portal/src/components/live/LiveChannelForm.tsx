import * as React from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import type { CreateLiveChannelRequest, LiveTestResult } from "@/types/live";
import {
  AlertCircle,
  CheckCircle2,
  FlaskConical,
  Loader2,
  Plus,
} from "lucide-react";

interface LiveChannelFormProps {
  resolvers: string[];
  onCreate: (req: CreateLiveChannelRequest) => Promise<unknown>;
  onTest: (req: CreateLiveChannelRequest) => Promise<LiveTestResult>;
}

const RESOLVER_HELP: Record<string, string> = {
  direct: "The source URL is already a playable .m3u8 or .mpd manifest.",
  api: "The source URL is a provider endpoint returning JSON; give the path to the manifest URL inside it.",
  static:
    "The source URL is a web page. Its HTML and inline scripts are scanned for a manifest.",
};

export function LiveChannelForm({
  resolvers,
  onCreate,
  onTest,
}: LiveChannelFormProps) {
  const [title, setTitle] = React.useState("");
  const [sourceUrl, setSourceUrl] = React.useState("");
  const [resolver, setResolver] = React.useState(resolvers[0] ?? "direct");
  const [jsonPath, setJsonPath] = React.useState("");
  const [referer, setReferer] = React.useState("");
  const [ttlSeconds, setTtlSeconds] = React.useState("");

  const [test, setTest] = React.useState<LiveTestResult | null>(null);
  const [isTesting, setIsTesting] = React.useState(false);
  const [isCreating, setIsCreating] = React.useState(false);
  const [formError, setFormError] = React.useState<string | null>(null);

  const buildRequest = React.useCallback((): CreateLiveChannelRequest => {
    const config: Record<string, unknown> = {};
    // Referer is a forbidden header in the browser, which is why hotlink-protected
    // origins can only be reached through the backend proxy at all.
    if (referer.trim()) config.headers = { Referer: referer.trim() };
    if (resolver === "api" && jsonPath.trim())
      config.json_path = jsonPath.trim();
    if (ttlSeconds.trim()) config.ttl_seconds = Number(ttlSeconds);

    return {
      title: title.trim(),
      resolver,
      source_url: sourceUrl.trim(),
      resolver_config: config,
      // Reported by the test probe rather than typed: an operator has no reliable
      // way to know a stream's window length by looking at it.
      is_dvr: !!test?.window_seconds && test.window_seconds > 30,
      dvr_window_seconds: Math.round(test?.window_seconds ?? 0),
    };
  }, [title, resolver, sourceUrl, referer, jsonPath, ttlSeconds, test]);

  const handleTest = async () => {
    setIsTesting(true);
    setFormError(null);
    try {
      setTest(await onTest(buildRequest()));
    } catch (err) {
      setFormError(err instanceof Error ? err.message : "Test failed");
      setTest(null);
    } finally {
      setIsTesting(false);
    }
  };

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsCreating(true);
    setFormError(null);
    try {
      await onCreate(buildRequest());
      setTitle("");
      setSourceUrl("");
      setJsonPath("");
      setReferer("");
      setTtlSeconds("");
      setTest(null);
    } catch (err) {
      setFormError(
        err instanceof Error ? err.message : "Could not create the channel",
      );
    } finally {
      setIsCreating(false);
    }
  };

  const canSubmit =
    title.trim() !== "" && sourceUrl.trim() !== "" && !isCreating;

  return (
    <form
      onSubmit={handleCreate}
      className="space-y-4 rounded-xl border border-zinc-800 bg-zinc-900/40 p-4"
    >
      <div className="grid gap-4 md:grid-cols-2">
        <div className="space-y-1.5">
          <Label htmlFor="live-title">Channel name</Label>
          <Input
            id="live-title"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="News 24 HD"
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="live-resolver">Source kind</Label>
          <select
            id="live-resolver"
            value={resolver}
            onChange={(e) => {
              setResolver(e.target.value);
              setTest(null);
            }}
            className="flex h-9 w-full rounded-md border border-zinc-800 bg-zinc-950 px-3 py-1 text-sm text-zinc-100 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-emerald-500"
          >
            {resolvers.map((id) => (
              <option key={id} value={id}>
                {id}
              </option>
            ))}
          </select>
          <p className="text-[11px] text-zinc-500">
            {RESOLVER_HELP[resolver] ?? "Custom resolver."}
          </p>
        </div>
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="live-source">
          {resolver === "static"
            ? "Page URL"
            : resolver === "api"
              ? "Provider API URL"
              : "Manifest URL"}
        </Label>
        <Input
          id="live-source"
          value={sourceUrl}
          onChange={(e) => {
            setSourceUrl(e.target.value);
            setTest(null);
          }}
          placeholder={
            resolver === "static"
              ? "https://example.com/watch/news24"
              : "https://cdn.example.com/live/master.m3u8"
          }
        />
        <p className="text-[11px] text-zinc-500">
          The host must be listed in{" "}
          <code className="text-zinc-400">LIVE_SOURCE_ALLOWED_HOSTS</code> on
          the backend.
        </p>
      </div>

      <div className="grid gap-4 md:grid-cols-3">
        {resolver === "api" && (
          <div className="space-y-1.5">
            <Label htmlFor="live-jsonpath">JSON path to the manifest URL</Label>
            <Input
              id="live-jsonpath"
              value={jsonPath}
              onChange={(e) => setJsonPath(e.target.value)}
              placeholder="data.streams.0.url"
            />
          </div>
        )}
        <div className="space-y-1.5">
          <Label htmlFor="live-referer">Referer sent upstream (optional)</Label>
          <Input
            id="live-referer"
            value={referer}
            onChange={(e) => setReferer(e.target.value)}
            placeholder="https://example.com/"
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="live-ttl">Re-resolve after (seconds, optional)</Label>
          <Input
            id="live-ttl"
            type="number"
            min={0}
            value={ttlSeconds}
            onChange={(e) => setTtlSeconds(e.target.value)}
            placeholder="3600"
          />
        </div>
      </div>

      {formError && (
        <p className="flex items-center gap-2 rounded-md border border-rose-500/25 bg-rose-500/10 px-3 py-2 font-mono text-xs text-rose-300">
          <AlertCircle className="h-3.5 w-3.5 shrink-0" />
          {formError}
        </p>
      )}

      {test && <TestResultPanel result={test} />}

      <div className="flex items-center gap-3">
        <Button
          type="button"
          variant="outline"
          onClick={handleTest}
          disabled={isTesting || !sourceUrl.trim()}
        >
          {isTesting ? (
            <Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" />
          ) : (
            <FlaskConical className="mr-2 h-3.5 w-3.5" />
          )}
          Test resolve
        </Button>
        <Button type="submit" disabled={!canSubmit}>
          {isCreating ? (
            <Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" />
          ) : (
            <Plus className="mr-2 h-3.5 w-3.5" />
          )}
          Create channel
        </Button>
        <span className="text-[11px] text-zinc-500">
          Test first — it reports the stream's real shape before anyone joins a
          room.
        </span>
      </div>
    </form>
  );
}

/** Shows what the resolver actually reached, so failures are diagnosable here. */
function TestResultPanel({ result }: { result: LiveTestResult }) {
  if (!result.ok) {
    return (
      <div className="rounded-md border border-rose-500/25 bg-rose-500/10 p-3">
        <p className="flex items-center gap-2 font-mono text-xs text-rose-300">
          <AlertCircle className="h-3.5 w-3.5 shrink-0" />
          {result.error || "The source could not be resolved."}
        </p>
      </div>
    );
  }

  return (
    <div className="rounded-md border border-emerald-500/25 bg-emerald-500/10 p-3 space-y-2">
      <p className="flex items-center gap-2 text-xs font-semibold text-emerald-300">
        <CheckCircle2 className="h-3.5 w-3.5" />
        Resolved to {result.manifest_host}
      </p>
      <div className="flex flex-wrap gap-2">
        <Badge variant="outline" className="text-[10px]">
          {result.protocol?.toUpperCase()}
        </Badge>
        <Badge
          variant={result.is_live ? "success" : "warning"}
          className="text-[10px]"
        >
          {result.is_live ? "LIVE WINDOW" : "ENDED (has EXT-X-ENDLIST)"}
        </Badge>
        {result.is_master && result.variants && result.variants.length > 0 && (
          <Badge variant="outline" className="text-[10px]">
            LADDER {result.variants.join(" · ")}
          </Badge>
        )}
        {!!result.target_duration && (
          <Badge variant="outline" className="text-[10px]">
            {result.target_duration}s SEGMENTS
          </Badge>
        )}
        {!!result.window_seconds && (
          <Badge variant="outline" className="text-[10px]">
            {Math.round(result.window_seconds)}s WINDOW
          </Badge>
        )}
        {result.expires_at && (
          <Badge variant="warning" className="text-[10px]">
            EXPIRES {new Date(result.expires_at).toLocaleTimeString()}
          </Badge>
        )}
      </div>
      {result.is_master && (
        <p className="font-mono text-[11px] text-zinc-400">
          Master playlist — viewers get adaptive bitrate switching.
        </p>
      )}
    </div>
  );
}
