import React, { useEffect, useRef } from "react";
import type { VoicePeer } from "../../types/rtc";

interface AudioRendererProps {
  peers: VoicePeer[];
  isDeafened: boolean;
}

// Browsers block audio.play() until the page has seen a user gesture. Relying
// on the `autoplay` attribute alone fails silently when a peer's track arrives
// asynchronously (e.g. right after joining a voice channel with no prior
// interaction elsewhere on the page), leaving remote audio muted with no error.
// Explicitly calling play() and retrying on the next gesture removes the need
// to "prime" playback via an unrelated element (like the video player) first.
function attemptPlay(audioEl: HTMLAudioElement): void {
  audioEl.play().catch(() => {
    const retry = () => {
      audioEl.play().catch(() => {});
    };
    document.addEventListener("pointerdown", retry, { once: true });
    document.addEventListener("keydown", retry, { once: true });
  });
}

/** One element per peer, mounted for as long as that peer is in the call. */
const PeerAudio: React.FC<{ stream: MediaStream; isDeafened: boolean }> = ({
  stream,
  isDeafened,
}) => {
  const ref = useRef<HTMLAudioElement | null>(null);

  useEffect(() => {
    const element = ref.current;
    if (!element || element.srcObject === stream) return;
    element.srcObject = stream;
    if (!isDeafened) attemptPlay(element);
  }, [stream, isDeafened]);

  useEffect(() => {
    const element = ref.current;
    if (!element) return;
    element.muted = isDeafened;
    if (!isDeafened) attemptPlay(element);
  }, [isDeafened]);

  return <audio ref={ref} autoPlay />;
};

/**
 * Plays everyone else's microphones.
 *
 * Keyed per peer rather than rebuilt as one block: recreating every element
 * whenever the roster changes cuts the audio of everyone already in the call
 * each time somebody joins.
 */
export const AudioRenderer: React.FC<AudioRendererProps> = ({
  peers,
  isDeafened,
}) => (
  <div style={{ display: "none" }}>
    {peers
      .filter((peer) => !peer.isLocal && peer.audioStream)
      .map((peer) => (
        <PeerAudio
          key={peer.userId}
          stream={peer.audioStream!}
          isDeafened={isDeafened}
        />
      ))}
  </div>
);
