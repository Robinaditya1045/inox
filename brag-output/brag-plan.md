# Brag Plan: Inox

## What is this app?
Inox is a synchronized watch-party platform — create a room, invite friends, and
watch video together with every playhead held on the same second, while talking
over voice chat and sharing screens. Go backend (REST + one WebSocket per room +
a Pion SFU), React viewer client, separate operator dashboard.

## The angle
Everyone already knows the ritual: "three, two, one — play," and then it falls
apart anyway. The video is about the thing that replaces the countdown. Not a
feature list — one honest mechanism, shown: the room has a single playhead, and
every follower quietly bends its own playback rate until it is back on it. The
numbers on screen are the real constants from `frontend/src/utils/liveSync.ts`,
not invented stats. That specificity is the whole angle — this is an engineering
product film for a product that actually has engineering in it.

## Hook (first 2-3 seconds)
A cold, near-black frame. `3` · `2` · `1` step in on the beat, then `play` —
the countdown everyone does out loud before starting a movie together. Motion is
immediate (first tick at ~0.4s), so the frame is alive before the first word.

## Key moments (the middle)
- Three scrubbers that started together visibly separating, tagged with their
  real drift: `+0.8s`, `+2.1s`, `-4.4s`.
- The actual Inox room: `# watch-party` header, the player with its real `SYNCED`
  pill, the scrubber, the chat rail reading `Message #general`, and the voice
  tiles along the bottom — one lighting up with the real speaking ring.
- The drift ladder itself: three follower rows showing measured drift and the
  action taken — hold inside the deadband, nudge the rate, seek past four
  seconds — then all three collapsing onto one playhead.

## Outro / punchline
The three drifted playheads converge to a single line, the wordmark lands on it:
**Inox — Same movie. Same second.**

## User flow worth showing
Entry → key action → result, as a real Inox session:
1. **Entry** — the room shell loads: `# watch-party`, chat rail, voice tiles.
2. **Key action** — somebody presses play; the `PLAY` event fans out over the
   room's single WebSocket.
3. **Result** — every follower converges. `SYNCED` holds, drift readings fall to
   the deadband, a voice tile rings as someone talks over it.

Scenes 3 and 4 are the centerpiece and both come from the working app, not from
marketing copy — the project has no landing page to recreate.

## Tone
- Preset: `polished`
- Creative direction: an engineering product film — the mechanism is the hero,
  delivered with restraint
- Interpretation: five scenes, long settled holds, no jokes and no hype words.
  Motion is confident rather than fast: things slide and settle, they do not
  bounce. The only "wow" the video reaches for is a viewer noticing the drift
  numbers are real.

## Format: landscape — 1920x1080, 30fps
## Duration: 24.04s (five scenes) — set by the generated narration, see Voiceover script

## Visual identity (from the project)
Pulled from `frontend/src/styles/theme.css`.
- Background: `#0a0d14` (`--color-canvas-deep`), panels `#0f1117` / `#161b27` /
  `#1a2035`
- Accent: `#4f73e0` (`--color-accent`), speaking ring `--color-speaking` (same
  blue) with a `rgba(79,115,224,0.12)` 4px halo
- Text: `#e8eaf0` primary, `#9aa3b5` secondary, `#6b7585` muted
- Success / live: `#2ecc71` — this is the `SYNCED` badge colour
- Danger (drift out of tolerance): `#e05656`
- Borders: `rgba(255,255,255,0.06 / 0.10 / 0.18)`
- Display + body font: **Figtree** (variable, 400–800) — the project's only
  typeface. Shipped as a local woff2 rather than linked from Google Fonts, so the
  render is deterministic and lint's `font_family_without_font_face` is satisfied.
  The drift readouts use JetBrains Mono, also shipped locally, standing in for the
  project's system `ui-monospace` stack.
- Radii: 6px tiles, 12px panels, 9999px pills
- Strongest visual element: the dark room shell — a green `SYNCED` pill and a
  blue speaking ring against a nearly black canvas.

## Share copy (draft)
Inox keeps a watch party on the same second — one playhead per room, and every
player bends its own speed until the drift is gone.

## Audio direction
- Role: warm, steady bed under a narrated product film; sparse professional accents
- Music: `happy-beats-business-moves-vol-12-by-ende-dot-app.mp3` (steady and
  clean, 109.96 BPM — the polished pick)
- Music treatment: starts at 0 at 0.30, ducked to **0.13** for the whole narrated
  stretch, back up to 0.30 after the last line so the tail rings under the
  wordmark, fading out over the final ~0.8s.
- Music cue guidance: preset read from
  `assets/music/cues/happy-beats-business-moves-vol-12-by-ende-dot-app.music-cues.json`.
  Tempo 109.96 BPM, beats ~0.545s apart.
  **Strong-cue locks (3, as built):** room reveal → **8.74s** (0.99); drift
  convergence → **19.66s** (0.96); wordmark landing → **22.37s** (0.98).
  **Beat-grid windows (as built):** countdown digits at 0.56 / 1.64 / 2.73 and
  `play` at 3.27; drift tags in Scene 2 at 5.34 / 6.00 / 6.56; drift-ladder rows
  in Scene 4 at 15.29 / 16.38 / 17.47 — deliberately **every other beat**
  (1.09s apart) because those rows are text that has to be read; outro lines at
  20.19 / 21.28.
- Audio-reactive treatment: subtle, and as built it drives two things: the light
  layer inside the player (opacity 0.34→0.84 on music RMS) and the presence of
  the speaking tile's halo (on bass). The glow originally planned *behind* the
  room panel was dropped — the room is full-bleed and opaque, so nothing behind
  it ever reaches a pixel. No waveform bars, no equalizers, no text scaling.
- SFX posture: sparse — 6 distinct moments (10 clips, since the countdown and the
  ladder rows are each a scored sequence of three) across 24.04s, all low HF
  risk, 0.42-0.68 volume. Warm `impactSoft_medium` family for reveals,
  `interface/click_003` for the play press, one `impactBell_heavy_000` on the
  convergence payoff.
- Audio-coupled moments: the countdown ticks; the play press; the room panel
  landing; the three drift-ladder rows arriving one by one; the convergence; the
  wordmark.
- Restraint rule: nothing bright, nothing hissy, nothing repeated more than
  three times, and anything that does land under a narration line is held at
  0.42 or below. The video must never sound busier than the product looks.

## Voiceover script
Kokoro via `hyperframes tts`, voice `af_heart`, speed 0.95. Five clips, one per
scene, on one track. The narration complements the frame — it never reads the
on-screen text aloud. Measured lengths drove the final scene durations:

| # | Scene | Line | Length | Placed |
|---|-------|------|--------|--------|
| 1 | Countdown | "Every watch party starts the same way." | 2.15s | 0.90 → 3.06 |
| 2 | Drift | "Then someone buffers, and nobody's in the same second." | 3.14s | 4.80 → 7.94 |
| 3 | The room | "Inox gives the whole room one playhead. Chat, playback, voice, one socket." | 4.95s | 9.20 → 14.15 |
| 4 | Drift ladder | "Each player measures its drift, then bends its own speed until it's gone." | 4.22s | 15.05 → 19.28 |
| 5 | Lockup | "Same movie. Same second. Inox." | 2.50s | 20.40 → 22.90 |

47 words of speech across 24.04s, with a 0.9–1.7s gap between every line. Each
line resolves before its scene cuts, and the music's duck lane breathes up into
each of those gaps rather than sitting flat under the whole film.

Kokoro needed `HYPERFRAMES_PYTHON` pointed at a venv holding `kokoro-onnx` and
`soundfile`; the CLI's own check reports it as not installed without that.

## Storyboard

### Scene 1 — The countdown — 4.39s (0.00 → 4.39)
Near-black `#0a0d14`. Centred, Figtree 800, `#e8eaf0`: `3`, then `2`, then `1`,
each replacing the last, then `play` in the accent blue `#4f73e0`. Beneath it a
single thin scrubber line, still. Nothing else on screen.
Sequential/interaction: yes — the three count digits step in one at a time
(1.09s apart, every other beat), each settling ~0.7s; every digit clears at the
exact instant the next thing lands, so no two numerals are ever crossfading on
top of each other. `play` lands at 3.27 and holds 1.1s.
Audio intent: quiet, patient, a room about to start something.
Audio-coupled idea: a soft warm tick per digit; a single `interface/click` on
`play`.
Music: steady bed, low.
Transition mood: clean → Scene 2

### Scene 2 — Drift — 4.35s (4.39 → 8.74)
The one scrubber becomes three stacked scrubber lines. They start flush, then
visibly separate. A mono drift tag settles beside each as it goes: `+0.8s`,
`+2.1s`, `-4.4s` — the last one in `#e05656`, the only warm colour in the frame.
Sequential/interaction: yes — the three tags arrive on beats 5.34 / 6.00 / 6.56
as the lines pull apart; they are short labels, not sentences, so the beat
spacing is safe here.
Audio intent: the quiet going slightly wrong — no alarm, just slippage.
Audio-coupled idea: one very soft accent per tag, thinning out; nothing on the
third.
Music: unchanged.
Transition mood: clean → Scene 3

### Scene 3 — The room — 5.99s (8.74 → 14.73)
The real Inox room shell, recreated in HTML at the project's own palette: left
rail and `# watch-party` channel header in `#161b27`, the player panel on
`#0f1117` with a green `SYNCED` pill (`#2ecc71`, wifi glyph) top-left of the
frame, a scrubber underneath, the chat rail at the right reading
`Message #general`, and four voice tiles along the bottom. One tile takes the
real speaking ring — 2px `#4f73e0` border plus the 4px `rgba(79,115,224,0.12)`
halo. Small caption, Figtree 600: **One room. One playhead.**
Sequential/interaction: yes — the panel lands first (**beat-locked 8.74s**), a
chat line arrives in the rail, then the voice tile rings.
Audio intent: arrival. The one moment in the video that is allowed weight.
Audio-coupled idea: `impact/impactSoft_medium_001` on the panel landing; a
whisper-quiet accent on the tile ring.
Music: bed holds; accent glow behind the panel may breathe with RMS.
Transition mood: soft → Scene 4

### Scene 4 — The drift ladder — 5.46s (14.73 → 20.19)
Close on the mechanism. Three follower rows, each a mono drift reading and the
action taken, in the project's own terms:
- `0.12s` → `hold` (inside the deadband)
- `0.90s` → `rate ×1.02`
- `5.20s` → `seek`
Then all three readings fall to `0.00s` and the three scrubber lines from Scene 2
collapse back onto one.
Sequential/interaction: yes — rows arrive at 15.29 / 16.38 / 17.47, **every other
beat**, 1.09s apart, so each row is settled and readable before the next; the full
set then holds together for 2.2s before the convergence at **19.66s**.
Audio intent: mechanical, calm, then resolved.
Audio-coupled idea: a dry low tick per row arrival; one `impactBell_heavy_000` at
the convergence — the only bell in the video. A vertical accent rule then draws
through the three now-aligned playheads.
Music: bed holds under the narration.
Transition mood: soft → Scene 5

### Scene 5 — Lockup — 3.85s (20.19 → 24.04)
The converged line carries over as a standing accent rule. **Same movie.** lands
on the cut (20.19) and **Same second.** a bar later (21.28), both in `#9aa3b5`;
then **Inox** lands above them at **22.37s** in Figtree 800 `#e8eaf0` with the
accent underline drawing under it. Tagline first, name last, because the
narration ends on the word "Inox" — the wordmark lands on the word.
Sequential/interaction: yes — three landings one bar apart, then stillness.
Audio intent: the music comes back up for the last beat and is the only thing
left.
Audio-coupled idea: `impact/impactSoft_medium_004` on the wordmark landing
(**beat-locked 22.37s**); nothing after it.
Music: duck released to 0.30 at 23.35, fade out over the final 0.44s.
Transition mood: fade to black

**Music mood for this video:** polished — steady, clean, unhurried
**Audio summary:** A low steady bed runs the whole film, ducked under five calm
narration lines; sparse warm accents mark the countdown, the room landing, the
three ladder rows and the wordmark, with a single bell at the moment the drift
resolves and the music rising back only for the final hold.
