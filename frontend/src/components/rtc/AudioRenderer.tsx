import React, { useEffect, useRef } from 'react';

interface AudioRendererProps {
  remoteStreams: Map<string, MediaStream>;
  isDeafened: boolean;
}

export const AudioRenderer: React.FC<AudioRendererProps> = ({ remoteStreams, isDeafened }) => {
  const containerRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!containerRef.current) return;

    // Clear existing audio nodes
    containerRef.current.innerHTML = '';

    remoteStreams.forEach((stream, peerId) => {
      const audioEl = document.createElement('audio');
      audioEl.id = `audio-peer-${peerId}`;
      audioEl.autoplay = true;
      audioEl.srcObject = stream;
      audioEl.muted = isDeafened;
      
      containerRef.current?.appendChild(audioEl);
    });
  }, [remoteStreams, isDeafened]);

  return <div ref={containerRef} style={{ display: 'none' }} />;
};
