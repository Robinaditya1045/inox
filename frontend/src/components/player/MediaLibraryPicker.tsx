import React, { useMemo, useState } from "react";
import { useMediaLibrary } from "../../hooks/useMediaLibrary";
import { useLiveChannels } from "../../hooks/useLiveChannels";
import { Modal } from "../common/Modal";
import { TextField } from "../common/TextField";
import { Button } from "../common/Button";
import { Skeleton } from "../common/Skeleton";
import { EmptyState } from "../common/EmptyState";
import { Tabs } from "../common/Tabs";
import type {
  LiveChannelStatus,
  LiveChannelSummary,
  MediaAsset,
} from "../../types/media";
import {
  AlertCircle,
  AppWindow,
  Braces,
  Check,
  Clock,
  CloudUpload,
  FileCode,
  Film,
  History,
  Link2,
  Play,
  Radio,
  RefreshCw,
  type LucideIcon,
} from "lucide-react";
import { stagger } from "../../utils/motion";
import { tabIds } from "../../utils/tabs";
import { isLiveMediaUrl } from "../../utils/liveSync";
import styles from "./MediaLibraryPicker.module.css";

interface MediaLibraryPickerProps {
  isOpen: boolean;
  onClose: () => void;
  currentUrl: string;
  onSelectUrl: (url: string) => void;
}

type PickerTab = "uploads" | "live" | "url";

const TAB_PREFIX = "media-picker";

/** How a live channel's stream is obtained, in viewer language. */
const RESOLVERS: Record<
  string,
  { label: string; hint: string; Icon: LucideIcon }
> = {
  browser: {
    label: "Browser",
    hint: "Captured by playing the source page in a headless browser",
    Icon: AppWindow,
  },
  api: {
    label: "API",
    hint: "Resolved through the provider's API",
    Icon: Braces,
  },
  static: {
    label: "Static",
    hint: "Found in the source page's HTML",
    Icon: FileCode,
  },
  direct: {
    label: "Direct",
    hint: "A stream URL used as-is",
    Icon: Link2,
  },
};

const STATUS: Record<
  LiveChannelStatus,
  { label: string; tone: "live" | "idle" | "warn" | "error" | "off" }
> = {
  live: { label: "Live", tone: "live" },
  idle: { label: "Standby", tone: "idle" },
  resolving: { label: "Connecting", tone: "warn" },
  degraded: { label: "Unstable", tone: "warn" },
  error: { label: "Offline", tone: "error" },
  disabled: { label: "Disabled", tone: "off" },
};

function formatDuration(seconds: number): string {
  if (!seconds || seconds === 0) return "—";
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = Math.floor(seconds % 60);
  if (h > 0)
    return `${h}:${m.toString().padStart(2, "0")}:${s.toString().padStart(2, "0")}`;
  return `${m}:${s.toString().padStart(2, "0")}`;
}

function getPlayUrl(asset: MediaAsset): string {
  // Prefer HLS master for ABR, fall back to source. Deliberately NOT normalized:
  // this URL is broadcast to every peer and persisted on the room, so it must stay
  // canonical. normalizeMediaUrl runs per-client at playback time instead.
  return asset.hls_master_url || asset.source_url;
}

function isLiveAsset(asset: MediaAsset): boolean {
  return asset.kind === "live" || isLiveMediaUrl(getPlayUrl(asset));
}

function getRenditionBadge(asset: MediaAsset): string | null {
  if (!asset.renditions || asset.renditions.length === 0) return null;
  const maxRes = Math.max(
    ...asset.renditions.map((r) => parseInt(r.resolution) || 0),
  );
  return maxRes > 0 ? `${maxRes}p` : null;
}

interface AssetOptionProps {
  asset: MediaAsset;
  index: number;
  isSelected: boolean;
  disabled?: boolean;
  /** Placeholder glyph when the asset has no thumbnail */
  FallbackIcon: LucideIcon;
  tags?: React.ReactNode;
  meta?: React.ReactNode;
  onSelect: () => void;
  onChoose: () => void;
}

/** One selectable row: a radio in the list's radiogroup. Double-click loads it. */
const AssetOption: React.FC<AssetOptionProps> = ({
  asset,
  index,
  isSelected,
  disabled,
  FallbackIcon,
  tags,
  meta,
  onSelect,
  onChoose,
}) => (
  <button
    type="button"
    role="radio"
    aria-checked={isSelected}
    aria-disabled={disabled || undefined}
    className={styles.asset}
    style={stagger(index)}
    onClick={() => !disabled && onSelect()}
    onDoubleClick={() => !disabled && onChoose()}
  >
    <span className={styles.thumb} aria-hidden="true">
      {asset.thumbnail_url ? (
        <img
          src={asset.thumbnail_url}
          alt=""
          width={64}
          height={36}
          loading="lazy"
        />
      ) : (
        <FallbackIcon size={18} />
      )}
    </span>

    <span className={styles.info}>
      <span className={styles.titleRow}>
        <span className={styles.title}>{asset.title}</span>
        {tags}
      </span>
      {meta && <span className={styles.meta}>{meta}</span>}
    </span>

    <span
      className={`${styles.indicator} ${isSelected ? styles.indicatorOn : ""}`}
      aria-hidden="true"
    >
      {isSelected ? <Check size={13} /> : <Play size={14} />}
    </span>
  </button>
);

const ListSkeleton: React.FC<{ label: string }> = ({ label }) => (
  <div aria-busy="true" aria-label={label}>
    {[0, 1, 2, 3].map((i) => (
      <div key={i} className={styles.skeletonRow}>
        <Skeleton width={64} height={36} radius="var(--radius-md)" />
        <div className={styles.skeletonLines}>
          <Skeleton width={`${58 - i * 8}%`} height={11} />
          <Skeleton width="28%" height={9} />
        </div>
      </div>
    ))}
  </div>
);

export const MediaLibraryPicker: React.FC<MediaLibraryPickerProps> = ({
  isOpen,
  onClose,
  currentUrl,
  onSelectUrl,
}) => {
  const { assets, isLoading, error, refresh } = useMediaLibrary();
  const { byAssetId: channels, refresh: refreshChannels } = useLiveChannels();
  const [customUrl, setCustomUrl] = useState("");
  const [selected, setSelected] = useState<string>(currentUrl);
  // Open on whichever kind of media the room is playing now.
  const [tab, setTab] = useState<PickerTab>(() =>
    isLiveMediaUrl(currentUrl) ? "live" : "uploads",
  );
  const [resolverFilter, setResolverFilter] = useState<string>("all");

  const { uploads, live } = useMemo(
    () => ({
      uploads: assets.filter((a) => !isLiveAsset(a)),
      live: assets.filter(isLiveAsset),
    }),
    [assets],
  );

  // Only offer a resolver filter when there is more than one kind to choose from.
  const resolversPresent = useMemo(() => {
    const seen = new Set<string>();
    for (const asset of live) {
      const resolver = channels.get(asset.id)?.resolver;
      if (resolver) seen.add(resolver);
    }
    return [...seen].sort();
  }, [live, channels]);

  const activeFilter =
    resolverFilter !== "all" && resolversPresent.includes(resolverFilter)
      ? resolverFilter
      : "all";
  const visibleLive =
    activeFilter === "all"
      ? live
      : live.filter((a) => channels.get(a.id)?.resolver === activeFilter);

  // "Load Stream" acts on what is selected in the tab you are looking at, not on
  // something picked earlier on the other tab.
  const selectedInTab =
    tab === "uploads"
      ? uploads.some((a) => getPlayUrl(a) === selected)
      : tab === "live"
        ? visibleLive.some(
            (a) =>
              getPlayUrl(a) === selected &&
              channels.get(a.id)?.status !== "disabled",
          )
        : false;

  const choose = (url: string) => {
    onSelectUrl(url);
    onClose();
  };

  const handleConfirm = () => {
    const target = tab === "url" ? customUrl.trim() : selected;
    if (target) choose(target);
  };

  const handleRefresh = () => {
    refresh();
    refreshChannels();
  };

  const panel = tabIds(TAB_PREFIX, tab);

  const countBadge = (n: number) =>
    n > 0 ? (
      <span className={`${styles.tag} ${styles.tagRes}`}>{n}</span>
    ) : null;

  const refreshButton = (
    <Button
      variant="ghost"
      size="sm"
      className={styles.refreshBtn}
      onClick={handleRefresh}
      disabled={isLoading}
      icon={
        <span className={styles.refreshIcon}>
          <RefreshCw size={13} />
        </span>
      }
    >
      Refresh
    </Button>
  );

  const loadError = error && (
    <EmptyState
      compact
      tone="danger"
      icon={<AlertCircle size={20} />}
      title="Couldn't load the library"
      description={error}
      action={
        <Button variant="secondary" size="sm" onClick={handleRefresh}>
          Try again
        </Button>
      }
    />
  );

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Media Library"
      description="Choose what the whole room watches next."
      maxWidth="600px"
    >
      <div className={styles.body}>
        <Tabs<PickerTab>
          className={styles.tabs}
          ariaLabel="Media source"
          idPrefix={TAB_PREFIX}
          value={tab}
          onChange={setTab}
          items={[
            {
              id: "uploads",
              label: "Uploads",
              icon: <CloudUpload size={14} />,
              badge: countBadge(uploads.length),
            },
            {
              id: "live",
              label: "Live",
              icon: <Radio size={14} />,
              badge: countBadge(live.length),
            },
            { id: "url", label: "Custom URL", icon: <Link2 size={14} /> },
          ]}
        />

        <div
          key={tab}
          className={styles.panel}
          role="tabpanel"
          id={panel.panel}
          aria-labelledby={panel.tab}
        >
          {tab === "uploads" && (
            <>
              <div className={styles.toolbar}>
                <span className={styles.caption}>
                  Uploaded & transcoded (ABR ready)
                </span>
                {refreshButton}
              </div>

              {isLoading ? (
                <ListSkeleton label="Loading uploads" />
              ) : error ? (
                loadError
              ) : uploads.length === 0 ? (
                <EmptyState
                  compact
                  icon={<Film size={20} />}
                  title="No uploads yet"
                  description="Upload and transcode videos from the admin portal, or paste a link under Custom URL."
                  action={
                    <Button
                      variant="secondary"
                      size="sm"
                      icon={<Link2 size={13} />}
                      onClick={() => setTab("url")}
                    >
                      Use a URL instead
                    </Button>
                  }
                />
              ) : (
                <div
                  className={styles.list}
                  role="radiogroup"
                  aria-label="Uploaded videos"
                >
                  {uploads.map((asset, i) => {
                    const playUrl = getPlayUrl(asset);
                    const qualityBadge = getRenditionBadge(asset);
                    return (
                      <AssetOption
                        key={asset.id}
                        asset={asset}
                        index={i}
                        isSelected={selected === playUrl}
                        FallbackIcon={Film}
                        onSelect={() => setSelected(playUrl)}
                        onChoose={() => choose(playUrl)}
                        tags={
                          <>
                            {asset.hls_master_url && (
                              <span
                                className={`${styles.tag} ${styles.tagHls}`}
                              >
                                HLS·ABR
                              </span>
                            )}
                            {qualityBadge && (
                              <span
                                className={`${styles.tag} ${styles.tagRes}`}
                              >
                                {qualityBadge}
                              </span>
                            )}
                          </>
                        }
                        meta={
                          <>
                            {asset.duration_seconds > 0 && (
                              <span className={styles.metaItem}>
                                <Clock size={10} aria-hidden="true" />
                                {formatDuration(asset.duration_seconds)}
                              </span>
                            )}
                            {asset.renditions &&
                              asset.renditions.length > 0 && (
                                <span>
                                  {asset.renditions.length} renditions
                                </span>
                              )}
                          </>
                        }
                      />
                    );
                  })}
                </div>
              )}
            </>
          )}

          {tab === "live" && (
            <>
              <div className={styles.toolbar}>
                {resolversPresent.length > 1 ? (
                  <div
                    className={styles.filters}
                    role="group"
                    aria-label="Filter by how the stream is sourced"
                  >
                    {["all", ...resolversPresent].map((id) => {
                      const meta = RESOLVERS[id];
                      const count =
                        id === "all"
                          ? live.length
                          : live.filter(
                              (a) => channels.get(a.id)?.resolver === id,
                            ).length;
                      return (
                        <button
                          key={id}
                          type="button"
                          className={styles.filterChip}
                          aria-pressed={activeFilter === id}
                          onClick={() => setResolverFilter(id)}
                        >
                          {meta && <meta.Icon size={12} aria-hidden="true" />}
                          {id === "all" ? "All" : (meta?.label ?? id)}
                          <span className={styles.filterCount}>{count}</span>
                        </button>
                      );
                    })}
                  </div>
                ) : (
                  <span className={styles.caption}>Live channels</span>
                )}
                {refreshButton}
              </div>

              {isLoading ? (
                <ListSkeleton label="Loading live channels" />
              ) : error ? (
                loadError
              ) : live.length === 0 ? (
                <EmptyState
                  compact
                  icon={<Radio size={20} />}
                  title="No live channels yet"
                  description="Channels added in the admin portal (from a browser capture, a provider API, a page, or a direct stream) show up here."
                />
              ) : (
                <div
                  className={styles.list}
                  role="radiogroup"
                  aria-label="Live channels"
                >
                  {visibleLive.map((asset, i) => {
                    const playUrl = getPlayUrl(asset);
                    const channel: LiveChannelSummary | undefined =
                      channels.get(asset.id);
                    const status = channel ? STATUS[channel.status] : undefined;
                    const resolver = channel
                      ? RESOLVERS[channel.resolver]
                      : undefined;
                    const disabled = channel?.status === "disabled";
                    return (
                      <AssetOption
                        key={asset.id}
                        asset={asset}
                        index={i}
                        isSelected={selected === playUrl}
                        disabled={disabled}
                        FallbackIcon={Radio}
                        onSelect={() => setSelected(playUrl)}
                        onChoose={() => choose(playUrl)}
                        tags={
                          status && (
                            <span
                              className={styles.status}
                              data-tone={status.tone}
                            >
                              <span
                                className={styles.statusDot}
                                aria-hidden="true"
                              />
                              {status.label}
                            </span>
                          )
                        }
                        meta={
                          channel && (
                            <>
                              <span
                                className={styles.metaItem}
                                title={resolver?.hint}
                              >
                                {resolver ? (
                                  <resolver.Icon size={11} aria-hidden="true" />
                                ) : null}
                                via {resolver?.label ?? channel.resolver}
                              </span>
                              {channel.is_dvr && (
                                <span className={styles.metaItem}>
                                  <History size={11} aria-hidden="true" />
                                  DVR
                                </span>
                              )}
                            </>
                          )
                        }
                      />
                    );
                  })}
                </div>
              )}
            </>
          )}

          {tab === "url" && (
            <>
              <TextField
                label="Video URL (MP4, WebM or HLS .m3u8)"
                type="url"
                name="media-url"
                inputMode="url"
                autoComplete="off"
                spellCheck={false}
                placeholder="https://example.com/video.mp4"
                value={customUrl}
                onChange={(e) => setCustomUrl(e.target.value)}
                icon={<Link2 size={16} />}
                onKeyDown={(e) => {
                  if (e.key === "Enter" && customUrl.trim()) handleConfirm();
                }}
              />
              <p className={styles.hint}>
                Paste any direct video link. HLS manifests (.m3u8) stream with
                adaptive bitrate through hls.js.
              </p>
            </>
          )}
        </div>

        <div className={styles.footer}>
          <Button type="button" variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button
            type="button"
            variant="primary"
            icon={<Play size={14} fill="currentColor" />}
            disabled={tab === "url" ? !customUrl.trim() : !selectedInTab}
            onClick={handleConfirm}
          >
            Load Stream
          </Button>
        </div>
      </div>
    </Modal>
  );
};
