import React, { useEffect, useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useRoom } from "../hooks/useRoom";
import { usePermissions } from "../hooks/usePermissions";
import { useRTC } from "../hooks/useRTC";
import { usePlayerSync } from "../hooks/usePlayerSync";
import { useRoomSocket } from "../hooks/useRoomSocket";
import { usePresence } from "../hooks/usePresence";
import { useMediaQuery } from "../hooks/useMediaQuery";
import { useAuth } from "../hooks/useAuth";
import { Button } from "../components/common/Button";
import { IconButton } from "../components/common/IconButton";
import { Skeleton } from "../components/common/Skeleton";
import { Spinner } from "../components/common/Spinner";
import { EmptyState } from "../components/common/EmptyState";
import { Tabs } from "../components/common/Tabs";
import { Badge } from "../components/common/Badge";
import { WatchPartyPlayer } from "../components/player/WatchPartyPlayer";
import { MediaLibraryPicker } from "../components/player/MediaLibraryPicker";
import { ChatPanel } from "../components/chat/ChatPanel";
import { MemberList } from "../components/room/MemberList";
import { AudioRenderer } from "../components/rtc/AudioRenderer";
import { RoomSidebar, type RoomChannel } from "../components/room/RoomSidebar";
import { RoomHeader, type SidePanelTab } from "../components/room/RoomHeader";
import { VoiceStage } from "../components/room/VoiceStage";
import { MessageSquare, RotateCcw, ShieldAlert, Users, X } from "lucide-react";
import { tabIds } from "../utils/tabs";
import shellStyles from "../components/room/VideoRoomShell.module.css";
import styles from "./RoomPage.module.css";

// The side panel (members / chat) is a column on desktop and a drawer below it;
// a drawer that opened by itself would cover the room on arrival.
const DESKTOP_QUERY = "(min-width: 1024px)";
const PANEL_TABS_ID = "room-panel";

export const RoomPage: React.FC = () => {
  const { roomId } = useParams<{ roomId: string }>();
  const { activeRoom, joinRoom, leaveRoom, roomError, clearRoomError } =
    useRoom();
  const permissions = usePermissions();
  const navigate = useNavigate();

  const [activeChannel, setActiveChannel] =
    useState<RoomChannel>("watch-party");
  const isDesktop = useMediaQuery(DESKTOP_QUERY);
  // Tracked separately so narrowing the window never pops a drawer open over
  // the room just because the column happened to be showing.
  const [showMemberColumn, setShowMemberColumn] = useState(true);
  const [isMemberDrawerOpen, setIsMemberDrawerOpen] = useState(false);
  const showMembers = isDesktop ? showMemberColumn : isMemberDrawerOpen;
  const setShowMembers = isDesktop
    ? setShowMemberColumn
    : setIsMemberDrawerOpen;
  const [isChannelDrawerOpen, setIsChannelDrawerOpen] = useState(false);
  const [isLibraryOpen, setIsLibraryOpen] = useState(false);
  // Counts openings. The picker mounts on the first one and remounts on each,
  // so its tab, selection and live statuses reflect the room when it opens —
  // and a room that never opens it never fetches the library.
  const [libraryOpens, setLibraryOpens] = useState(0);
  const openLibrary = () => {
    setLibraryOpens((n) => n + 1);
    setIsLibraryOpen(true);
  };

  // The side panel holds the member list and a second view of #general, so the
  // conversation stays in reach while watching or in a call. When #general is
  // already the main view the chat tab would only duplicate it.
  const [panelTab, setPanelTab] = useState<SidePanelTab>("members");
  const chatInMain = activeChannel === "general";
  const effectivePanelTab: SidePanelTab = chatInMain ? "members" : panelTab;
  const openPanel: SidePanelTab | null = showMembers ? effectivePanelTab : null;
  const isChatVisible = chatInMain || openPanel === "chat";
  // Stamped with the room it counts for: this page is reused across room
  // switches, and a count from the previous room must not show in the next.
  const [unread, setUnread] = useState<{ roomId?: string; count: number }>({
    count: 0,
  });
  const chatUnread = unread.roomId === roomId ? unread.count : 0;
  const clearChatUnread = () => setUnread({ roomId, count: 0 });
  const { user } = useAuth();

  // The URL is the source of truth for which room we want. activeRoom may still be the
  // PREVIOUS room while a switch is in flight, so gate everything on this instead.
  const isRoomReady = !!activeRoom && activeRoom.id === roomId;

  // Key the socket and the SFU off the requested room, not the stale active one —
  // otherwise switching rooms leaves you on the old room's socket.
  const { subscribe } = useRoomSocket(roomId);
  const rtc = useRTC(roomId);
  const { setMediaUrl, mediaUrl, isPlaying } = usePlayerSync();
  const { members } = usePresence();

  useEffect(() => {
    if (!roomId) return;
    // A failure joining a previous room must not condemn this one.
    clearRoomError();
    joinRoom(roomId).catch(() => {});
    // Intentionally keyed on roomId alone: re-running on activeRoom would re-join
    // the room we just successfully joined.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [roomId]);

  // Escape closes whichever drawer is open (only drawers exist below desktop).
  const anyDrawerOpen =
    isChannelDrawerOpen || (!isDesktop && isMemberDrawerOpen);
  const closeDrawers = () => {
    setIsChannelDrawerOpen(false);
    setIsMemberDrawerOpen(false);
  };
  useEffect(() => {
    if (!anyDrawerOpen) return;
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key !== "Escape") return;
      setIsChannelDrawerOpen(false);
      setIsMemberDrawerOpen(false);
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [anyDrawerOpen]);

  // Count messages from others that land while no chat is on screen. The badge is
  // cleared by whatever brings a chat back into view (the handlers below).
  useEffect(() => {
    if (isChatVisible) return;
    return subscribe("CHAT_MESSAGE", (msg) => {
      // A frame still in flight from the previous room's socket is not ours.
      if (msg.room_id && msg.room_id !== roomId) return;
      if (msg.sender_id && msg.sender_id !== user?.id) {
        setUnread((u) => ({
          roomId,
          count: (u.roomId === roomId ? u.count : 0) + 1,
        }));
      }
    });
  }, [isChatVisible, subscribe, user?.id, roomId]);

  const togglePanel = (tab: SidePanelTab) => {
    if (openPanel === tab) {
      setShowMembers(false);
      return;
    }
    setPanelTab(tab);
    setShowMembers(true);
    if (tab === "chat") clearChatUnread();
  };

  const selectPanelTab = (tab: SidePanelTab) => {
    setPanelTab(tab);
    if (tab === "chat") clearChatUnread();
  };

  const handleLeave = async () => {
    await leaveRoom();
    navigate("/");
  };

  const handleSelectChannel = (channel: RoomChannel) => {
    setActiveChannel(channel);
    setIsChannelDrawerOpen(false);
    if (channel === "general") clearChatUnread();
    // Selecting voice while disconnected joins it — the channel IS the call.
    if (
      channel === "voice" &&
      rtc.connectionState === "disconnected" &&
      permissions.can_stream_audio
    ) {
      rtc.connectAudio().catch(() => {});
    }
  };

  // ── Joining ───────────────────────────────────────────────────────────
  // Checked before the error branch and covering the first paint, so a hard refresh
  // no longer flashes "Room not accessible" before the join has even started.
  if (!isRoomReady && !roomError) {
    return (
      <div className={styles.joining} aria-busy="true">
        <div className={styles.joiningSidebar} aria-hidden="true">
          <Skeleton className={styles.joiningHeader} width="70%" height={16} />
          {[64, 48, 72].map((w) => (
            <Skeleton key={w} width={`${w}%`} height={12} />
          ))}
        </div>
        <div className={styles.joiningMain}>
          <div className={styles.joiningBar} aria-hidden="true">
            <Skeleton width={140} height={14} />
          </div>
          <div className={styles.joiningStage} role="status">
            <Spinner size={22} label={null} />
            Joining room…
          </div>
        </div>
      </div>
    );
  }

  // ── Join failed ───────────────────────────────────────────────────────
  if (roomError) {
    return (
      <div className={styles.state}>
        <EmptyState
          as="h1"
          tone="danger"
          icon={<ShieldAlert size={24} />}
          title="Room not accessible"
          description={roomError}
          action={
            <>
              <Button variant="secondary" onClick={() => navigate("/")}>
                Return to Lobby
              </Button>
              <Button
                icon={<RotateCcw size={14} />}
                onClick={() => {
                  clearRoomError();
                  if (roomId) joinRoom(roomId).catch(() => {});
                }}
              >
                Try again
              </Button>
            </>
          }
        />
      </div>
    );
  }

  // ── Room ──────────────────────────────────────────────────────────────
  return (
    <>
      {/* aria-live region for voice events */}
      <div
        id="room-announcer"
        aria-live="polite"
        aria-atomic="false"
        className="sr-only"
      />

      <div
        className={`${shellStyles.shell} ${showMembers ? shellStyles.shellPanelOpen : ""}`}
      >
        <div
          className={`${shellStyles.roomSidebarArea} ${isChannelDrawerOpen ? shellStyles.roomSidebarOpen : ""}`}
        >
          <RoomSidebar
            activeRoom={activeRoom}
            activeChannel={activeChannel}
            onSelectChannel={handleSelectChannel}
            onLeave={handleLeave}
            isMediaPlaying={isPlaying}
          />
        </div>

        <div className={shellStyles.workspaceArea}>
          <RoomHeader
            activeRoom={activeRoom}
            activeChannel={activeChannel}
            openPanel={openPanel}
            onTogglePanel={togglePanel}
            showChatToggle={!chatInMain}
            chatUnread={chatUnread}
            onLeave={handleLeave}
            onOpenChannels={() => setIsChannelDrawerOpen(true)}
            mediaUrl={mediaUrl}
            memberCount={members.length}
          />

          {activeChannel === "general" && (
            <div key="general" className={shellStyles.channelView}>
              <ChatPanel roomId={activeRoom?.id} />
            </div>
          )}

          {activeChannel === "voice" && (
            <div key="voice" className={shellStyles.channelView}>
              <VoiceStage roomId={activeRoom?.id} />
            </div>
          )}

          {activeChannel === "watch-party" && (
            <div key="watch-party" className={shellStyles.channelView}>
              <div className={shellStyles.playerArea}>
                <WatchPartyPlayer
                  onOpenLibrary={
                    permissions.can_control_playback ? openLibrary : undefined
                  }
                />
              </div>
            </div>
          )}
        </div>

        <aside
          className={shellStyles.panelArea}
          aria-label={effectivePanelTab === "chat" ? "Chat" : "Members"}
          aria-hidden={!showMembers || undefined}
        >
          <div className={shellStyles.panelBar}>
            {chatInMain ? (
              <span className={shellStyles.panelTitle}>Members</span>
            ) : (
              <Tabs<SidePanelTab>
                variant="pill"
                ariaLabel="Side panel"
                idPrefix={PANEL_TABS_ID}
                value={effectivePanelTab}
                onChange={selectPanelTab}
                className={shellStyles.panelTabs}
                items={[
                  {
                    id: "members",
                    label: "Members",
                    icon: <Users size={14} />,
                  },
                  {
                    id: "chat",
                    label: "Chat",
                    icon: <MessageSquare size={14} />,
                    badge:
                      chatUnread > 0 ? (
                        <Badge variant="danger" pop>
                          {chatUnread > 99 ? "99+" : chatUnread}
                        </Badge>
                      ) : null,
                  },
                ]}
              />
            )}
            <IconButton
              label="Close side panel"
              className={shellStyles.panelClose}
              onClick={() => setShowMembers(false)}
              tooltip={false}
            >
              <X size={18} />
            </IconButton>
          </div>

          {/* The member list stays mounted while chat is showing: it tracks joins
              and leaves from the socket and would lose them if it unmounted. */}
          <div
            className={shellStyles.panelBody}
            hidden={effectivePanelTab !== "members"}
            {...(chatInMain
              ? {}
              : {
                  role: "tabpanel",
                  id: tabIds(PANEL_TABS_ID, "members").panel,
                  "aria-labelledby": tabIds(PANEL_TABS_ID, "members").tab,
                })}
          >
            <MemberList />
          </div>
          {effectivePanelTab === "chat" && (
            <div
              className={shellStyles.panelBody}
              role="tabpanel"
              id={tabIds(PANEL_TABS_ID, "chat").panel}
              aria-labelledby={tabIds(PANEL_TABS_ID, "chat").tab}
            >
              <ChatPanel roomId={activeRoom?.id} compact />
            </div>
          )}
        </aside>

        <div
          className={`${shellStyles.backdrop} ${anyDrawerOpen ? shellStyles.backdropVisible : ""}`}
          onClick={closeDrawers}
          aria-hidden="true"
        />
      </div>

      {/* Invisible Audio Renderer */}
      <AudioRenderer peers={rtc.peers} isDeafened={rtc.isDeafened} />

      {/* Media Library Modal — stays mounted after closing so it can animate out */}
      {libraryOpens > 0 && (
        <MediaLibraryPicker
          key={libraryOpens}
          isOpen={isLibraryOpen}
          onClose={() => setIsLibraryOpen(false)}
          currentUrl={mediaUrl}
          onSelectUrl={setMediaUrl}
        />
      )}
    </>
  );
};
