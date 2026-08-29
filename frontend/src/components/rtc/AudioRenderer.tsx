import React, { useEffect, useRef } from "react";

interface AudioRendererProps {
  remoteStreams: Map<string, MediaStream>;
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

export const AudioRenderer: React.FC<AudioRendererProps> = ({
  remoteStreams,
  isDeafened,
}) => {
  const containerRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!containerRef.current) return;

    // Clear existing audio nodes
    containerRef.current.innerHTML = "";

    remoteStreams.forEach((stream, peerId) => {
      const audioEl = document.createElement("audio");
      audioEl.id = `audio-peer-${peerId}`;
      audioEl.autoplay = true;
      audioEl.srcObject = stream;
      audioEl.muted = isDeafened;

      containerRef.current?.appendChild(audioEl);

      if (!isDeafened) {
        attemptPlay(audioEl);
      }
    });
  }, [remoteStreams, isDeafened]);

  return <div ref={containerRef} style={{ display: "none" }} />;
};
