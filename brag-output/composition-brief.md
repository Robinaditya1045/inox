# Hyperframes Composition Brief: Inox

## Objective
Create a short, narrated launch-style brag video for Inox — a synchronized
watch-party platform. The film is about one mechanism, shown honestly: the room
has a single playhead, and every follower bends its own playback rate until it is
back on it.

## Output
- Composition directory: `brag-output/composition/`
- Rendered video: `brag-output/brag.mp4`
- Format: landscape — 1920x1080, 30fps
- Duration: **24.04s as built** (the planned 23.5s flexed to the generated narration)

## Source Material
- Project root: `/home/robin/code/go/inox`
- Primary files read: `README.md`, `frontend/src/styles/theme.css`,
  `frontend/src/utils/liveSync.ts`,
  `frontend/src/components/player/WatchPartyPlayer.tsx`,
  `frontend/src/components/room/VoiceStage.tsx` + `VoiceStage.module.css`,
  `frontend/src/components/room/RoomHeader.tsx`,
  `frontend/src/components/chat/ChatPanel.tsx`,
  `backend/internal/sfu/room.go`, `backend/internal/media/processor.go`
- Product name: **Inox**
- Tagline / strongest claim: *Same movie. Same second.* — one playhead per room,
  held by a drift ladder rather than by pausing everybody.
- Key UI to recreate: the Inox room shell — `# watch-party` channel header, the
  player panel with its green `SYNCED` pill, the scrubber, the chat rail, and the
  voice tiles with the blue speaking ring.

### Copy that must appear verbatim (all of it is real product text or real constants)
- `SYNCED` — the player's live status pill (`WatchPartyPlayer.tsx:536`)
- `watch-party` — the channel name, rendered as `# watch-party`
- `Message #general` — the chat composer placeholder (`ChatPanel.tsx:209`)
- `0.12s` / `hold`, `0.90s` / `rate ×1.02`, `5.20s` / `seek` — the three rungs of
  the real drift ladder (`liveSync.ts`: deadband `0.15`, gentle/firm rate steps,
  seek past `4`)
- `+0.8s`, `+2.1s`, `-4.4s` — the pre-sync drift tags
- `Inox`, `Same movie. Same second.`

## Creative Direction
- Tone preset: `polished`
- Creative direction: an engineering product film — the mechanism is the hero,
  delivered with restraint
- Interpretation: five scenes, long settled holds, no hype words and no jokes.
  Motion is confident, not fast: elements slide and settle, nothing bounces or
  overshoots hard. The payoff the video reaches for is a viewer noticing that the
  numbers on screen are the product's actual constants.
- Angle: everyone knows the "three, two, one — play" ritual, and everyone knows
  it does not hold. This video is about what replaces it. The centerpiece is the
  working app, not a feature list — Inox has no landing page to recreate, so
  every scene after the hook comes from the product itself.
- Hook: a cold near-black frame; `3` · `2` · `1` step in on the beat, then `play`.
- Outro / punchline: three drifted playheads collapse to one line, the wordmark
  lands on it — **Inox — Same movie. Same second.**
- Avoid:
  - Generic SaaS language ("streamline", "seamless", "supercharge")
  - Abstract filler visuals, particle fields, colour washes
  - Redesigning Inox — use its own palette, type and components
  - Inventing metrics; every number on screen is sourced above

## Visual Identity
From `frontend/src/styles/theme.css` (the project's design tokens).
- Background: `#0a0d14` (canvas-deep); panels `#0f1117`, `#161b27`, `#1a2035`
- Text: `#e8eaf0` primary, `#9aa3b5` secondary, `#6b7585` muted
- Accent: `#4f73e0`; subtle `rgba(79,115,224,0.12)`; border `rgba(79,115,224,0.35)`
- Success / live (the `SYNCED` pill): `#2ecc71`
- Danger (out-of-tolerance drift): `#e05656`
- Borders: `rgba(255,255,255,0.06 / 0.10 / 0.18)`
- Display + body font: **Figtree** (variable 400–800), the project's only
  typeface. Shipped as a local woff2 under `assets/fonts/` rather than linked
  from Google Fonts, so the render is deterministic and lint's
  `font_family_without_font_face` is satisfied. Drift readouts use JetBrains Mono,
  also shipped locally, standing in for the project's `ui-monospace` stack.
- Radii: 6px tiles, 12px panels, 9999px pills
- Visual references from the project: the speaking tile ring (2px `#4f73e0` +
  4px `rgba(79,115,224,0.12)` halo), the `SYNCED` pill with its wifi glyph, the
  16:9 voice tiles, the channel header with its `#` glyph.

Contrast note: `#9aa3b5` on `#0a0d14` and `#e8eaf0` on `#0f1117` both clear
WCAG AA comfortably. `#6b7585` muted text is borderline — do not use it for any
copy the viewer must read; keep it for decorative chrome only, or lift it to
`#9aa3b5`. Fix whatever `check` reports within this palette family.

## Storyboard
`brag-output/brag-plan.md` holds the creative contract — follow its scene
descriptions, sequential-reveal commitments and reading holds.

Scene summary:
1. **The countdown** — 4.39s (0.00 → 4.39) — `3`, `2`, `1` step in one at a time,
   then `play` in accent blue over a single still scrubber line.
2. **Drift** — 4.35s (4.39 → 8.74) — one scrubber becomes three; they separate;
   mono tags `+0.8s`, `+2.1s`, `-4.4s` settle beside them (the last in `#e05656`).
3. **The room** — 5.99s (8.74 → 14.73) — the Inox room shell: `# watch-party`
   header, player panel with the green `SYNCED` pill and scrubber, chat rail
   reading `Message #general`, four voice tiles with one taking the speaking
   ring. Caption: **One room. One playhead.**
4. **The drift ladder** — 5.46s (14.73 → 20.19) — three follower rows arrive one
   by one: `0.12s → hold`, `0.90s → rate ×1.02`, `5.20s → seek`; then all three
   readings fall to `0.00s` and a rule draws through the three aligned playheads.
5. **Lockup** — 3.85s (20.19 → 24.04) — **Same movie.** / **Same second.** land a
   bar apart, then **Inox** lands above them at 22.37s with the accent underline;
   hold, fade.

## Audio
- Audio role: warm steady bed under a narrated product film, with sparse
  professional accents.
- Audio arc: low bed from frame one, ducked under five calm narration lines,
  accents marking only the countdown / room landing / ladder rows / convergence /
  wordmark, then the music released back up to carry the final hold alone.
- Music: `assets/music/happy-beats-business-moves-vol-12-by-ende-dot-app.mp3`
  (steady and clean, 109.96 BPM).
- Music treatment: `data-start="0"` with a `data-automation` volume lane. As
  built the duck *breathes* rather than sitting flat: 0.13 under each of the five
  narration lines, up to 0.26-0.30 in the four gaps between them (so the room
  landing and the convergence both hit against a fuller bed), released to 0.30 at
  23.35 and faded out by 24.04.
- Music cue guidance: preset at
  `assets/music/cues/happy-beats-business-moves-vol-12-by-ende-dot-app.music-cues.json`.
  Beats ~0.545s apart.
  - **Strong-cue locks (3, as built):** room panel landing → **8.74s** (0.99);
    drift convergence → **19.66s** (0.96); wordmark landing → **22.37s** (0.98).
    Each marked `// beat-locked` in the source.
  - **Beat-grid windows (as built):** countdown at **0.56 / 1.64 / 2.73** with
    `play` on **3.27**; Scene 2 drift tags at **5.34 / 6.00 / 6.56**; Scene 4
    ladder rows at **15.29 / 16.38 / 17.47**; outro lines at **20.19 / 21.28**.
  - Scene 4's rows are **text the viewer must read** — they are deliberately
    spaced every *other* beat (1.09s apart), not every beat. Do not tighten them.
    As built the full set holds for 2.2s after the third row before the convergence.
- Audio-reactive treatment: **subtle**, and as built it drives exactly two
  things: a light layer *inside* the player (opacity 0.34→0.84 on music RMS) and
  the presence of the speaking tile's halo (on bass). The glow originally planned
  *behind* the room panel was dropped: the room is full-bleed and opaque, so
  nothing behind it is ever visible. Extracted with the `hyperframes-creative`
  workflow's `extract-audio-data.py` and sampled as a pure function of timeline
  time, so a seek and a forward render agree. No waveform bars, no equalizers, no
  particles, no text scaling.
- Audio-coupled moments:
  - Scene 1 countdown digits — a soft warm tick per digit; `interface/click_003`
    on `play`
  - Scene 3 room panel landing — the one moment allowed weight
    (`impact/impactSoft_medium_001`)
  - Scene 3 voice tile ring — whisper-quiet accent
  - Scene 4 ladder rows — a dry low tick at each row's arrival, same timestamp as
    the visual
  - Scene 4 convergence — `impact/impactBell_heavy_000`, the only bell in the film
  - Scene 5 wordmark — `impact/impactSoft_medium_004`, then nothing
- SFX selection guidance: 5–7 cues total across 23.5s, volumes **0.55–0.70**.
  Everything stays warm and low-HF; nothing repeats more than three times.
  Already copied into `assets/sfx/`: `impact/impactSoft_medium_000`, `_001`,
  `_004`, `impact/impactBell_heavy_000`, `interface/click_003`,
  `interface/drop_001`, `drop_002`. Swap within the low-HF families if the
  implemented motion wants something else — copy any new file in first.
- SFX analysis guidance:
  `/home/robin/.claude/plugins/cache/brag/brag/0.2.2/skills/brag/assets/sfx/sfx-analysis.md`.
  This is a `polished` film — low HF risk only, no bright or hissy files.
- Exact SFX choice: Hyperframes picks final filenames, timestamps, density and
  volume once the animation exists.
- Restraint rule: no SFX may land on top of a narration line except the quiet
  row-arrival ticks in Scene 4.

### Voiceover (explicitly requested — `/brag --voice`)
Kokoro via `npx hyperframes tts`, voice `af_heart`, speed 0.95. Note: this machine
needs `HYPERFRAMES_PYTHON=~/.cache/hyperframes-tts-venv/bin/python` set for the
TTS command to find `kokoro-onnx` (see the venv note at the bottom).

Five clips, one per scene, written to `assets/vo/vo-1.wav` … `vo-5.wav`, placed on
their own track:

1. "Every watch party starts the same way."
2. "Then someone buffers — and nobody's in the same second."
3. "Inox gives the whole room one playhead. Chat, playback, voice — one socket."
4. "Each player measures its drift, then bends its own speed until it's gone."
5. "Same movie. Same second. Inox."

The narration complements the frame and never reads on-screen text aloud.
**Scene durations flexed to the generated audio.** Measured: 2.15 / 3.14 / 4.95 /
4.22 / 2.50s. Scene 3 grew from 5.46s to 5.99s to hold its 4.95s line, and the
convergence and wordmark locks moved out to the next strong cues (19.66s, 22.37s)
rather than squeezing a line. Every line resolves before its scene cuts.

Environment note: Kokoro is not installed by default here — `hyperframes doctor`
reports `TTS (Kokoro) Not installed`. It was provisioned into
`~/.cache/hyperframes-tts-venv` (`uv pip install kokoro-onnx soundfile`) and
reached via `HYPERFRAMES_PYTHON`.

## Hyperframes Instructions
Load `hyperframes-core` (composition contract + `data-*` timing),
`hyperframes-animation` (motion), `hyperframes-creative` (design spec, beats,
audio-reactive), `hyperframes-keyframes` (seek-safe keyframes),
`hyperframes-audio` (ducking / automation) and `hyperframes-cli` (check, beats,
render). `/brag` is its own workflow — do not enter the `hyperframes`
entry-point intent interview and do not route into the generic promo /
launch-video workflow. Prefer native Hyperframes conventions over anything here.

Requirements:
- Show the real Inox room UI (Scene 3) and the real drift-ladder values (Scene 4).
- Keep every text element readable: short labels settle ~0.8s, sentences ~0.3s
  per word. Scene 4's rows are the tightest constraint — respect their spacing.
- Total duration 15–25s.
- Include music, the planned SFX layer and the voiceover track.
- Treat the `/brag` audio notes as guidance, not a fixed cue sheet — choose SFX
  after the visual animation exists.
- Treat cue metadata as optional timing hints; ignore a cue that hurts
  readability or the product story.
- Use exactly the three strong-cue locks listed above; mark them in the source.
- Use local assets only — never absolute paths.
- Run `npx hyperframes check` before render; it is brag's single gate. Fix every
  error it reports, including WCAG contrast findings (see the contrast note under
  Visual Identity).
