import React, { useEffect, useState } from "react";
import type { CaptionCue } from "../../utils/captions";

interface CaptionOverlayProps {
  videoRef: React.RefObject<HTMLVideoElement | null>;
  cues: CaptionCue[];
  className: string;
  lineClassName: string;
}

/**
 * Draws the captions over the picture. Not a native <track>: that needs the
 * video element in CORS mode, which breaks playback of linked files on hosts
 * without CORS, and native cues sit under the custom control bar. Polled per
 * frame rather than on timeupdate (~4 Hz), so lines appear on time; state only
 * changes when the visible lines do.
 */
export const CaptionOverlay: React.FC<CaptionOverlayProps> = ({
  videoRef,
  cues,
  className,
  lineClassName,
}) => {
  const [lines, setLines] = useState<string[]>([]);

  useEffect(() => {
    if (cues.length === 0) return;
    let raf = 0;
    let shown = "";
    const tick = () => {
      const t = videoRef.current?.currentTime ?? 0;
      const next = cues
        .filter((c) => t >= c.start && t < c.end)
        .flatMap((c) => c.lines)
        .filter((l) => l.trim() !== "");
      const key = next.join("\n");
      if (key !== shown) {
        shown = key;
        setLines(next);
      }
      raf = requestAnimationFrame(tick);
    };
    raf = requestAnimationFrame(tick);
    return () => {
      cancelAnimationFrame(raf);
      setLines([]);
    };
  }, [cues, videoRef]);

  if (cues.length === 0 || lines.length === 0) return null;
  return (
    <div className={className}>
      {lines.map((line, i) => (
        <span key={i} className={lineClassName}>
          {line}
        </span>
      ))}
    </div>
  );
};
