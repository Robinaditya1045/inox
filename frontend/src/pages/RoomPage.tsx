import React, { useEffect, useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useRoom } from "../hooks/useRoom";
import { usePermissions } from "../hooks/usePermissions";
import { useRTC } from "../hooks/useRTC";
import { usePlayerSync } from "../hooks/usePlayerSync";
import { useRoomSocket } from "../hooks/useRoomSocket";
import { usePresence } from "../hooks/usePresence";
import { Button } from "../components/common/Button";
import { Spinner } from "../components/common/Spinner";
import { WatchPartyPlayer } from "../components/player/WatchPartyPlayer";
import { MediaLibraryPicker } from "../components/player/MediaLibraryPicker";
import { ChatPanel } from "../components/chat/ChatPanel";
import { MemberList } from "../components/room/MemberList";
import { AudioRenderer } from "../components/rtc/AudioRenderer";
import { RoomSidebar, type RoomChannel } from "../components/room/RoomSidebar";
import { RoomHeader } from "../components/room/RoomHeader";
import { VoiceStage } from "../components/room/VoiceStage";
import { Shield } from "lucide-react";
import shellStyles from "../components/room/VideoRoomShell.module.css";

export const RoomPage: React.FC = () => {
  const { roomId } = useParams<{ roomId: string }>();
  const { activeRoom, joinRoom, leaveRoom, roomError, clearRoomError } =
    useRoom();
  const permissions = usePermissions();
  const navigate = useNavigate();

  const [activeChannel, setActiveChannel] =
    useState<RoomChannel>("watch-party");
  const [showMembers, setShowMembers] = useState(true);
  const [isLibraryOpen, setIsLibraryOpen] = useState(false);

  // The URL is the source of truth for which room we want. activeRoom may still be the
  // PREVIOUS room while a switch is in flight, so gate everything on this instead.
  const isRoomReady = !!activeRoom && activeRoom.id === roomId;

  // Key the socket and the SFU off the requested room, not the stale active one —
  // otherwise switching rooms leaves you on the old room's socket.
  useRoomSocket(roomId);
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

  const handleLeave = async () => {
    await leaveRoom();
    navigate("/");
  };

  const handleSelectChannel = (channel: RoomChannel) => {
    setActiveChannel(channel);
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
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          height: "100%",
          flexDirection: "column",
          gap: "var(--space-3)",
        }}
      >
        <Spinner size={28} />
        <span
          style={{
            color: "var(--color-text-secondary)",
            fontSize: "var(--text-compact)",
          }}
        >
          Joining room…
        </span>
      </div>
    );
  }

  // ── Join failed ───────────────────────────────────────────────────────
  if (roomError) {
    return (
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          height: "100%",
          flexDirection: "column",
          gap: "var(--space-4)",
          padding: "var(--space-8)",
          textAlign: "center",
        }}
      >
        <div
          style={{
            width: 44,
            height: 44,
            borderRadius: "50%",
            background: "var(--color-danger-subtle)",
            color: "var(--color-danger)",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
          }}
        >
          <Shield size={22} />
        </div>
        <div>
          <h2
            style={{
              fontSize: "var(--text-heading)",
              fontWeight: 700,
              color: "var(--color-text-primary)",
              marginBottom: "var(--space-2)",
            }}
          >
            Room not accessible
          </h2>
          <p
            style={{
              color: "var(--color-text-secondary)",
              maxWidth: 360,
              fontSize: "var(--text-compact)",
            }}
          >
            {roomError}
          </p>
        </div>
        <div style={{ display: "flex", gap: "var(--space-2)" }}>
          <Button
            variant="primary"
            onClick={() => {
              clearRoomError();
              if (roomId) joinRoom(roomId).catch(() => {});
            }}
          >
            Try again
          </Button>
          <Button variant="secondary" onClick={() => navigate("/")}>
            Return to Lobby
          </Button>
        </div>
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
        <div className={shellStyles.roomSidebarArea}>
          <RoomSidebar
            activeRoom={activeRoom}
            activeChannel={activeChannel}
            onSelectChannel={handleSelectChannel}
            isMediaPlaying={isPlaying}
          />
        </div>

        <div className={shellStyles.workspaceArea}>
          <RoomHeader
            activeRoom={activeRoom}
            activeChannel={activeChannel}
            showMembers={showMembers}
            onToggleMembers={() => setShowMembers((p) => !p)}
            onLeave={handleLeave}
            mediaUrl={mediaUrl}
            memberCount={members.length}
          />

          {activeChannel === "general" && <ChatPanel roomId={activeRoom?.id} />}

          {activeChannel === "voice" && <VoiceStage roomId={activeRoom?.id} />}

          {activeChannel === "watch-party" && (
            <div className={shellStyles.playerArea}>
              <WatchPartyPlayer
                onOpenLibrary={
                  permissions.can_control_playback
                    ? () => setIsLibraryOpen(true)
                    : undefined
                }
              />
            </div>
          )}
        </div>

        {showMembers && (
          <div className={shellStyles.panelArea}>
            <MemberList />
          </div>
        )}
      </div>

      {/* Invisible Audio Renderer */}
      <AudioRenderer peers={rtc.peers} isDeafened={rtc.isDeafened} />

      {/* Media Library Modal */}
      <MediaLibraryPicker
        isOpen={isLibraryOpen}
        onClose={() => setIsLibraryOpen(false)}
        currentUrl={mediaUrl}
        onSelectUrl={setMediaUrl}
      />
    </>
  );
};
