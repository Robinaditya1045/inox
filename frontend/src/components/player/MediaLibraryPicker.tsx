import React, { useMemo, useState } from "react";
import { useMediaLibrary } from "../../hooks/useMediaLibrary";
import { useLiveChannels } from "../../hooks/useLiveChannels";
import { usePlaylists } from "../../hooks/usePlaylists";
import { useToast } from "../../hooks/useToast";
import { Modal } from "../common/Modal";
import { TextField } from "../common/TextField";
import { Button } from "../common/Button";
import { Skeleton } from "../common/Skeleton";
import { EmptyState } from "../common/EmptyState";
import { Tabs } from "../common/Tabs";
import { MediaCard } from "../library/MediaCard";
import { SaveToPlaylistMenu } from "../library/SaveToPlaylistMenu";
import { NewPlaylistCard, PlaylistCard } from "../library/PlaylistCard";
import { PlaylistDetail } from "../library/PlaylistDetail";
import { PlaylistFormModal } from "../library/PlaylistFormModal";
import { ContinueWatching } from "./ContinueWatching";
import type { RoomArrival } from "../../contexts/playerSync.context";
import type {
  LiveChannelStatus,
  LiveChannelSummary,
  MediaAsset,
} from "../../types/media";
import type { WSQueue } from "../../types/ws";
import {
  AlertCircle,
  AppWindow,
  Braces,
  CloudUpload,
  FileCode,
  Film,
  History,
  Link2,
  ListVideo,
  Play,
  Radio,
  RefreshCw,
  Search,
  type LucideIcon,
} from "lucide-react";
import { tabIds } from "../../utils/tabs";
import { isLiveMediaUrl } from "../../utils/liveSync";
import {
  getPlayUrl,
  getResolutionLabel,
  isLiveAsset,
  queueFromPlaylist,
} from "../../utils/mediaAsset";
import styles from "./MediaLibraryPicker.module.css";

interface MediaLibraryPickerProps {
  isOpen: boolean;
  onClose: () => void;
  currentUrl: string;
  onSelectUrl: (url: string) => void;
  /** Starts the room playing through a playlist */
  onPlayQueue: (queue: WSQueue) => void;
  /**
   * Set when the picker opens because this viewer opened the room: what the
   * room was left on is offered above the library.
   */
  arrival?: RoomArrival | null;
  /** Plays the room from `time` in what it was left on */
  onResume?: (time: number) => void;
}

type PickerTab = "library" | "playlists" | "live" | "url";
type SortOrder = "newest" | "title" | "longest";

const TAB_PREFIX = "media-picker";

const SORTS: { id: SortOrder; label: string }[] = [
  { id: "newest", label: "Newest" },
  { id: "title", label: "A–Z" },
  { id: "longest", label: "Longest" },
];

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

const relativeDate = new Intl.RelativeTimeFormat(undefined, {
  numeric: "auto",
});

/** "3 days ago", "last month": how long a video has been in the library. */
function addedAgo(iso: string): string {
  const days = Math.round((Date.now() - new Date(iso).getTime()) / 86_400_000);
  if (!Number.isFinite(days)) return "";
  if (days < 1) return "Today";
  if (days < 7) return relativeDate.format(-days, "day");
  if (days < 30) return relativeDate.format(-Math.round(days / 7), "week");
  if (days < 365) return relativeDate.format(-Math.round(days / 30), "month");
  return relativeDate.format(-Math.round(days / 365), "year");
}

const GridSkeleton: React.FC<{ label: string }> = ({ label }) => (
  <div className={styles.grid} aria-busy="true" aria-label={label}>
    {[0, 1, 2, 3, 4, 5, 6, 7].map((i) => (
      <div key={i} className={styles.skeletonCard}>
        <Skeleton
          className={styles.skeletonPoster}
          height="auto"
          radius="var(--radius-lg)"
        />
        <Skeleton width={`${78 - (i % 3) * 14}%`} height={12} />
        <Skeleton width="40%" height={10} />
      </div>
    ))}
  </div>
);

export const MediaLibraryPicker: React.FC<MediaLibraryPickerProps> = ({
  isOpen,
  onClose,
  currentUrl,
  onSelectUrl,
  onPlayQueue,
  arrival,
  onResume,
}) => {
  const { assets, isLoading, error, refresh } = useMediaLibrary();
  const { byAssetId: channels, refresh: refreshChannels } = useLiveChannels();
  const playlistsApi = usePlaylists();
  const { playlists } = playlistsApi;
  const { toast } = useToast();

  const [customUrl, setCustomUrl] = useState("");
  const [selected, setSelected] = useState<string>(currentUrl);
  // Open on whichever kind of media the room is playing now — or, for someone
  // arriving, on the library under what the room was left on.
  const [tab, setTab] = useState<PickerTab>(() =>
    !arrival && isLiveMediaUrl(currentUrl) ? "live" : "library",
  );
  const [query, setQuery] = useState("");
  const [sort, setSort] = useState<SortOrder>("newest");
  const [resolverFilter, setResolverFilter] = useState<string>("all");
  const [openPlaylistId, setOpenPlaylistId] = useState<string | null>(null);

  // The create / edit dialog. pendingAsset is the video that "New playlist" in
  // a card's save menu was opened from: it goes straight into the new playlist.
  const [form, setForm] = useState<{
    mode: "create" | "edit";
    pendingAsset?: MediaAsset;
    key: number;
  } | null>(null);

  const { uploads, live } = useMemo(
    () => ({
      uploads: assets.filter((a) => !isLiveAsset(a)),
      live: assets.filter(isLiveAsset),
    }),
    [assets],
  );

  const visibleUploads = useMemo(() => {
    const q = query.trim().toLowerCase();
    const matched = q
      ? uploads.filter(
          (a) =>
            a.title.toLowerCase().includes(q) ||
            a.description?.toLowerCase().includes(q),
        )
      : uploads;
    const sorted = [...matched];
    if (sort === "title") {
      sorted.sort((a, b) => a.title.localeCompare(b.title));
    } else if (sort === "longest") {
      sorted.sort(
        (a, b) => (b.duration_seconds || 0) - (a.duration_seconds || 0),
      );
    }
    // "newest" is the server's order.
    return sorted;
  }, [uploads, query, sort]);

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

  const openPlaylist = playlists.find((p) => p.id === openPlaylistId) ?? null;

  // "Play" acts on what is selected in the tab you are looking at, not on
  // something picked earlier on another tab.
  const selectedInTab =
    tab === "library"
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

  const playPlaylist = (startIndex: number) => {
    if (!openPlaylist) return;
    const queue = queueFromPlaylist(openPlaylist, startIndex);
    if (!queue) return;
    onPlayQueue(queue);
    onClose();
  };

  const resume = (time: number) => {
    // A live channel has no room-wide play; it is already on in the player.
    if (arrival?.leftOn?.kind !== "live") onResume?.(time);
    onClose();
  };

  const handleConfirm = () => {
    const target = tab === "url" ? customUrl.trim() : selected;
    if (target) choose(target);
  };

  const handleRefresh = () => {
    refresh();
    refreshChannels();
    playlistsApi.refresh();
  };

  const submitForm = async (name: string, description: string) => {
    if (form?.mode === "edit" && openPlaylist) {
      return !!(await playlistsApi.update(openPlaylist.id, name, description));
    }
    const created = await playlistsApi.create(name, description);
    if (!created) return false;
    if (form?.pendingAsset) {
      await playlistsApi.addItem(created.id, form.pendingAsset);
    } else {
      // Made from the Playlists tab: open it, ready to be filled.
      setOpenPlaylistId(created.id);
      toast({ title: `Created ${created.name}`, tone: "success" });
    }
    return true;
  };

  const deleteOpenPlaylist = async () => {
    if (!openPlaylist) return;
    const name = openPlaylist.name;
    if (await playlistsApi.remove(openPlaylist.id)) {
      setOpenPlaylistId(null);
      toast({ title: `Deleted ${name}` });
    }
  };

  const panel = tabIds(TAB_PREFIX, tab);

  const countBadge = (n: number) =>
    n > 0 ? <span className={styles.countBadge}>{n}</span> : null;

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

  const saveMenu = (asset: MediaAsset) => (
    <SaveToPlaylistMenu
      asset={asset}
      playlists={playlists}
      onToggle={playlistsApi.toggleItem}
      onCreateNew={(a) =>
        setForm({ mode: "create", pendingAsset: a, key: Date.now() })
      }
    />
  );

  const showFooter = tab !== "playlists";

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={arrival ? "What are we watching?" : "Media Library"}
      description={
        arrival
          ? arrival.leftOn
            ? "You're the first one here. Pick up where the room left off, or choose something new."
            : "You're the first one here. Choose what the room watches."
          : "Choose what the whole room watches next."
      }
      maxWidth="1080px"
    >
      <div className={styles.body}>
        {arrival?.leftOn && onResume && (
          <ContinueWatching
            leftOn={arrival.leftOn}
            assets={assets}
            onResume={resume}
          />
        )}

        <Tabs<PickerTab>
          className={styles.tabs}
          ariaLabel="Media source"
          idPrefix={TAB_PREFIX}
          value={tab}
          onChange={setTab}
          items={[
            {
              id: "library",
              label: "Library",
              icon: <CloudUpload size={14} />,
              badge: countBadge(uploads.length),
            },
            {
              id: "playlists",
              label: "Playlists",
              icon: <ListVideo size={14} />,
              badge: countBadge(playlists.length),
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
          {tab === "library" && (
            <>
              <div className={styles.toolbar}>
                <div className={styles.search}>
                  <TextField
                    type="search"
                    name="library-search"
                    aria-label="Search the library"
                    placeholder="Search videos"
                    autoComplete="off"
                    value={query}
                    onChange={(e) => setQuery(e.target.value)}
                    icon={<Search size={15} />}
                  />
                </div>
                <div
                  className={styles.filters}
                  role="group"
                  aria-label="Sort videos"
                >
                  {SORTS.map((s) => (
                    <button
                      key={s.id}
                      type="button"
                      className={styles.filterChip}
                      aria-pressed={sort === s.id}
                      onClick={() => setSort(s.id)}
                    >
                      {s.label}
                    </button>
                  ))}
                </div>
                {refreshButton}
              </div>

              {isLoading ? (
                <GridSkeleton label="Loading the library" />
              ) : error ? (
                loadError
              ) : uploads.length === 0 ? (
                <EmptyState
                  compact
                  icon={<Film size={20} />}
                  title="No videos yet"
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
              ) : visibleUploads.length === 0 ? (
                <EmptyState
                  compact
                  icon={<Search size={20} />}
                  title="No matches"
                  description={`Nothing in the library matches “${query.trim()}”.`}
                  action={
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={() => setQuery("")}
                    >
                      Clear search
                    </Button>
                  }
                />
              ) : (
                <div
                  className={styles.grid}
                  role="radiogroup"
                  aria-label="Videos"
                >
                  {visibleUploads.map((asset, i) => {
                    const playUrl = getPlayUrl(asset);
                    const quality = getResolutionLabel(asset);
                    const added = addedAgo(asset.created_at);
                    return (
                      <MediaCard
                        key={asset.id}
                        asset={asset}
                        index={i}
                        isSelected={selected === playUrl}
                        isNowPlaying={currentUrl === playUrl}
                        onSelect={() => setSelected(playUrl)}
                        onPlay={() => choose(playUrl)}
                        badges={
                          quality && (
                            <span className={styles.chip}>{quality}</span>
                          )
                        }
                        meta={
                          <>
                            {asset.hls_master_url && <span>Adaptive</span>}
                            {added && <span>{added}</span>}
                          </>
                        }
                        actions={saveMenu(asset)}
                      />
                    );
                  })}
                </div>
              )}
            </>
          )}

          {tab === "playlists" &&
            (openPlaylist ? (
              <PlaylistDetail
                key={openPlaylist.id}
                playlist={openPlaylist}
                currentUrl={currentUrl}
                onBack={() => setOpenPlaylistId(null)}
                onPlay={playPlaylist}
                onEdit={() => setForm({ mode: "edit", key: Date.now() })}
                onDelete={deleteOpenPlaylist}
                onRemoveItem={(itemId) =>
                  playlistsApi.removeItem(openPlaylist.id, itemId)
                }
                onMoveItem={(itemId, by) =>
                  playlistsApi.moveItem(openPlaylist.id, itemId, by)
                }
                onBrowseLibrary={() => setTab("library")}
              />
            ) : (
              <>
                <div className={styles.toolbar}>
                  <span className={styles.caption}>Your playlists</span>
                  {refreshButton}
                </div>
                {playlistsApi.isLoading ? (
                  <GridSkeleton label="Loading your playlists" />
                ) : playlistsApi.error ? (
                  <EmptyState
                    compact
                    tone="danger"
                    icon={<AlertCircle size={20} />}
                    title="Couldn't load your playlists"
                    description={playlistsApi.error}
                    action={
                      <Button
                        variant="secondary"
                        size="sm"
                        onClick={playlistsApi.refresh}
                      >
                        Try again
                      </Button>
                    }
                  />
                ) : (
                  <>
                    <div className={styles.grid}>
                      <NewPlaylistCard
                        index={0}
                        onClick={() =>
                          setForm({ mode: "create", key: Date.now() })
                        }
                      />
                      {playlists.map((playlist, i) => (
                        <PlaylistCard
                          key={playlist.id}
                          playlist={playlist}
                          index={i + 1}
                          onOpen={() => setOpenPlaylistId(playlist.id)}
                        />
                      ))}
                    </div>
                    {playlists.length === 0 && (
                      <p className={styles.hint}>
                        Playlists play in order, moving on to the next video by
                        themselves. Make one here, or save videos to one from
                        the Library tab.
                      </p>
                    )}
                  </>
                )}
              </>
            ))}

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
                <GridSkeleton label="Loading live channels" />
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
                  className={styles.grid}
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
                    return (
                      <MediaCard
                        key={asset.id}
                        asset={{ ...asset, kind: "live" }}
                        index={i}
                        isSelected={selected === playUrl}
                        isNowPlaying={currentUrl === playUrl}
                        disabled={channel?.status === "disabled"}
                        onSelect={() => setSelected(playUrl)}
                        onPlay={() => choose(playUrl)}
                        badges={
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
            <div className={styles.urlPanel}>
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
            </div>
          )}
        </div>

        {showFooter && (
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
              Play for everyone
            </Button>
          </div>
        )}
      </div>

      {form && (
        <PlaylistFormModal
          key={form.key}
          isOpen
          onClose={() => setForm(null)}
          initial={
            form.mode === "edit" && openPlaylist
              ? {
                  name: openPlaylist.name,
                  description: openPlaylist.description,
                }
              : undefined
          }
          context={
            form.pendingAsset
              ? `“${form.pendingAsset.title}” will be its first video.`
              : undefined
          }
          onSubmit={submitForm}
        />
      )}
    </Modal>
  );
};
