import { useEffect, useRef, useState } from "react";

export interface SpeakingSource {
  id: string;
  stream: MediaStream;
}

/** RMS above which a stream counts as active speech rather than room noise. */
const SPEAKING_THRESHOLD = 0.035;
/** How long a tile keeps its ring after the last loud sample, so pauses between
 *  words do not make it strobe. */
const RELEASE_MS = 600;
const SAMPLE_INTERVAL_MS = 120;

function sameMembers(a: Set<string>, b: Set<string>): boolean {
  if (a.size !== b.size) return false;
  for (const id of a) {
    if (!b.has(id)) return false;
  }
  return true;
}

/**
 * Reports which of the given streams are currently carrying speech.
 *
 * The SFU forwards packets whether or not anyone is talking, so "who is speaking"
 * has to be measured from the audio itself. One AudioContext drives every source,
 * sampled ~8 times a second, which is fast enough to feel live and cheap enough to
 * leave running for the length of a call.
 */
export function useSpeakingPeers(sources: SpeakingSource[]): Set<string> {
  const [speaking, setSpeaking] = useState<Set<string>>(() => new Set());

  // The array is rebuilt every render; only its membership should restart the
  // analysers, so the effect keys off a signature instead of the array itself.
  const signature = sources
    .map((source) => `${source.id}:${source.stream.id}`)
    .sort()
    .join("|");
  const sourcesRef = useRef(sources);
  // Declared before the effect below so the latest streams are in place by the
  // time a membership change restarts the analysers.
  useEffect(() => {
    sourcesRef.current = sources;
  });

  useEffect(() => {
    const active = sourcesRef.current;
    if (active.length === 0) {
      setSpeaking((prev) => (prev.size === 0 ? prev : new Set()));
      return;
    }

    const AudioCtor: typeof AudioContext | undefined =
      window.AudioContext ??
      (window as unknown as { webkitAudioContext?: typeof AudioContext })
        .webkitAudioContext;
    if (!AudioCtor) return;

    const context = new AudioCtor();
    void context.resume().catch(() => {});

    const meters = active.flatMap(({ id, stream }) => {
      if (stream.getAudioTracks().length === 0) return [];
      try {
        const analyser = context.createAnalyser();
        analyser.fftSize = 512;
        analyser.smoothingTimeConstant = 0.2;
        const node = context.createMediaStreamSource(stream);
        node.connect(analyser);
        return [
          {
            id,
            analyser,
            node,
            buffer: new Float32Array(analyser.fftSize),
            lastLoudAt: 0,
          },
        ];
      } catch {
        return [];
      }
    });

    const timer = window.setInterval(() => {
      const now = Date.now();
      const next = new Set<string>();

      for (const meter of meters) {
        meter.analyser.getFloatTimeDomainData(meter.buffer);
        let sum = 0;
        for (let i = 0; i < meter.buffer.length; i += 1) {
          sum += meter.buffer[i] * meter.buffer[i];
        }
        const rms = Math.sqrt(sum / meter.buffer.length);
        if (rms > SPEAKING_THRESHOLD) meter.lastLoudAt = now;
        if (now - meter.lastLoudAt < RELEASE_MS) next.add(meter.id);
      }

      setSpeaking((prev) => (sameMembers(prev, next) ? prev : next));
    }, SAMPLE_INTERVAL_MS);

    return () => {
      window.clearInterval(timer);
      meters.forEach((meter) => {
        meter.node.disconnect();
        meter.analyser.disconnect();
      });
      void context.close().catch(() => {});
    };
  }, [signature]);

  return speaking;
}
