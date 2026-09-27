import { Badge } from "@/components/ui/badge";
import type { ResolveDiagnostics, ResolveOutcome } from "@/types/live";

type BadgeVariant =
  | "default"
  | "outline"
  | "success"
  | "warning"
  | "destructive"
  | "cyan"
  | "purple";

const OUTCOME: Record<
  ResolveOutcome,
  { label: string; variant: BadgeVariant }
> = {
  STREAM_FOUND: { label: "STREAM FOUND", variant: "success" },
  DRM_PROTECTED: { label: "DRM PROTECTED", variant: "purple" },
  SERVER_REPLAY_FAILED: { label: "BROWSER-ONLY", variant: "cyan" },
  NO_STREAM_FOUND: { label: "NO STREAM FOUND", variant: "warning" },
  RESOLUTION_TIMEOUT: { label: "TIMED OUT", variant: "warning" },
  RESOLUTION_FAILED: { label: "FAILED", variant: "destructive" },
};

/** Plain-language versions of the backend's reason codes. */
const REASON: Record<string, string> = {
  manifest_protected: "the manifest declares DRM",
  renditions_protected: "some renditions need DRM",
  eme_session: "the player set up DRM playback",
  only_clear_stream_ended:
    "the only unprotected stream was an advert or preview",
  clear_stream_alongside_drm: "DRM is also in use on the page",
  drm_suspected: "the player asked for DRM",
  no_manifest_requested: "the player requested no stream",
  no_manifest_in_page: "no manifest in the page source",
  pattern_unmatched: "nothing matched manifest_pattern",
  not_a_manifest: "no HLS or DASH manifest",
  page_unresponsive: "the page did not respond",
  page_still_loading: "the page was still loading",
  browser_busy: "the browser queue was full",
  verification_timeout: "checking the manifests timed out",
  deadline_exceeded: "ran out of time",
  page_http_error: "the page returned an HTTP error",
  page_not_found: "the page URL 404s — most likely stale or wrong",
  page_forbidden: "the page is access-controlled (401/403)",
  page_rate_limited: "the site is rate-limiting us (429)",
  page_server_error: "the site's own server is failing (5xx)",
  page_unreachable: "the page could not be reached",
  address_blocked: "a private or internal address",
  host_not_allowed: "a host outside LIVE_SOURCE_ALLOWED_HOSTS",
  upstream_refused: "the upstream refused the server",
  bot_protected: "a bot-protection check guards the stream",
  browser_bound:
    "the stream is tied to the browser session and can't be restreamed",
  manifest_load_failed: "the manifest failed to load in the browser",
  browser_unavailable: "the headless browser is unavailable",
  invalid_config: "a configuration mistake",
  resolver_error: "the resolver failed",
};

const VERDICT_STYLE: Record<string, string> = {
  chosen: "text-emerald-400",
  usable: "text-zinc-300",
  drm_protected: "text-purple-400",
  refused: "text-rose-400",
  not_allowed: "text-rose-400",
  browser_only: "text-cyan-400",
  not_loaded: "text-amber-400",
};

export function OutcomeBadge({
  outcome,
  reason,
}: {
  outcome: ResolveOutcome;
  reason?: string;
}) {
  const { label, variant } = OUTCOME[outcome] ?? {
    label: outcome,
    variant: "outline" as const,
  };
  return (
    <span className="inline-flex flex-wrap items-center gap-1.5">
      <Badge variant={variant} className="text-[10px]">
        {label}
      </Badge>
      {reason && (
        <span className="text-[11px] text-zinc-400" title={reason}>
          {REASON[reason] ?? reason}
        </span>
      )}
    </span>
  );
}

/**
 * What a resolve saw, so an operator can tell why a channel does or does not play.
 * Everything here is safe to show by construction: the backend stores hosts, counts
 * and DRM system names, never URLs, tokens or licence data.
 */
export function ResolveDiagnosticsView({
  diagnostics: d,
}: {
  diagnostics: ResolveDiagnostics;
}) {
  const facts: string[] = [];
  if (d.resolver) facts.push(`${d.resolver} resolver`);
  if (d.elapsed_ms) facts.push(`${(d.elapsed_ms / 1000).toFixed(1)}s`);
  if (d.page) {
    const p = d.page;
    if (p.host) facts.push(p.status ? `${p.host} → HTTP ${p.status}` : p.host);
    facts.push(p.loaded ? "page loaded" : "page still loading");
    facts.push(`${p.frames} cross-origin frame${p.frames === 1 ? "" : "s"}`);
    if (p.nudges)
      facts.push(`${p.nudges} play attempt${p.nudges === 1 ? "" : "s"}`);
    if (p.popups_closed) facts.push(`${p.popups_closed} popup(s) closed`);
    if (p.cookies)
      facts.push(
        `${p.cookies} session cookie${p.cookies === 1 ? "" : "s"} carried over`,
      );
    if (p.cookies_withheld)
      facts.push(
        p.cookies_withheld_names?.length
          ? `${p.cookies_withheld} bot-protection cookie(s) withheld (${p.cookies_withheld_names.join(", ")})`
          : `${p.cookies_withheld} bot-protection cookie(s) withheld`,
      );
  }

  const trails: { label: string; hosts?: string[] }[] = [
    { label: "Redirected through", hosts: d.page?.redirects },
    { label: "Frames", hosts: d.page?.frame_hosts },
    { label: "Cookie domains", hosts: d.page?.cookie_domains },
  ];

  return (
    <div className="space-y-2 font-mono text-[11px] text-zinc-400">
      {facts.length > 0 && <p>{facts.join(" · ")}</p>}

      {trails.map(
        ({ label, hosts }) =>
          !!hosts?.length && (
            <p key={label}>
              <span className="text-zinc-500">{label}:</span>{" "}
              {hosts.join(" · ")}
            </p>
          ),
      )}

      {d.drm && (
        <div className="space-y-1 rounded-md border border-purple-500/20 bg-purple-500/5 p-2">
          <p className="text-purple-300">
            {d.drm.confirmed
              ? "DRM confirmed"
              : "DRM suspected — the page only asked what the browser supports"}
            {d.drm.systems?.length ? `: ${d.drm.systems.join(", ")}` : ""}
            {d.drm.schemes?.length ? ` (${d.drm.schemes.join(", ")})` : ""}
          </p>
          {!!d.drm.renditions && (
            <p>
              {d.drm.protected_renditions ?? 0} of {d.drm.renditions} renditions
              protected
            </p>
          )}
          {!!d.drm.key_systems?.length && (
            <p>
              EME key systems requested: {d.drm.key_systems.join(", ")}
              {d.drm.active_key_systems?.length
                ? ` · in use: ${d.drm.active_key_systems.join(", ")}`
                : ""}
            </p>
          )}
          {!!d.drm.license_hosts?.length && (
            <p>Licence servers: {d.drm.license_hosts.join(", ")}</p>
          )}
          {!!d.drm.signals?.length && (
            <div className="flex flex-wrap gap-1">
              {d.drm.signals.map((s) => (
                <span
                  key={s}
                  className="rounded border border-zinc-700 px-1 py-px text-[10px] text-zinc-400"
                >
                  {s}
                </span>
              ))}
            </div>
          )}
        </div>
      )}

      {!!d.manifests?.length && (
        <ul className="space-y-0.5">
          {d.manifests.map((m, i) => (
            <li key={`${m.host}-${i}`}>
              <span className="text-zinc-300">{m.host}</span>
              {m.protocol && ` · ${m.protocol}`}
              {m.kind && ` ${m.kind}`}
              {m.protocol && (m.live ? " · live" : " · ended")}
              {" · "}
              <span className={VERDICT_STYLE[m.verdict] ?? "text-zinc-400"}>
                {m.verdict.replace(/_/g, " ")}
              </span>
              {m.drm?.length ? ` (${m.drm.join(", ")})` : ""}
              {!!m.browser_cookies &&
                ` · browser sent ${m.browser_cookies} cookie${m.browser_cookies === 1 ? "" : "s"}`}
              {m.detail && <span className="text-zinc-500"> — {m.detail}</span>}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
