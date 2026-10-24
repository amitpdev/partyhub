---
name: Party Hub
description: A violet-first, low-light control system for live party playback.
colors:
  night-ground: "#07070b"
  working-surface: "#111019"
  raised-surface: "#1b1928"
  control-edge: "#48425c"
  primary-ink: "#f8f7ff"
  secondary-ink: "#bbb7cb"
  tertiary-ink: "#9a95ab"
  signal-violet: "#9b87ff"
  signal-violet-ink: "#ddd5ff"
  signal-violet-surface: "#241b49"
  positive-ink: "#8bd8a8"
  error-ink: "#f06a72"
  warning-ink: "#e5a93b"
typography:
  title:
    fontFamily: "Inter Variable, Inter, system-ui, sans-serif"
    fontSize: "20px"
    fontWeight: 650
    lineHeight: 1.2
    letterSpacing: "-0.02em"
  body:
    fontFamily: "Inter Variable, Inter, system-ui, sans-serif"
    fontSize: "15px"
    fontWeight: 400
    lineHeight: 1.45
  label:
    fontFamily: "Inter Variable, Inter, system-ui, sans-serif"
    fontSize: "13px"
    fontWeight: 550
    lineHeight: 1.4
rounded:
  control: "999px"
  field: "14px"
  row: "12px"
  panel: "20px"
spacing:
  xs: "6px"
  sm: "10px"
  md: "14px"
  lg: "22px"
  xl: "28px"
components:
  button-primary:
    backgroundColor: "{colors.primary-ink}"
    textColor: "{colors.night-ground}"
    rounded: "{rounded.control}"
    height: "44px"
    padding: "0 17px"
  panel:
    backgroundColor: "{colors.working-surface}"
    textColor: "{colors.primary-ink}"
    rounded: "{rounded.panel}"
    padding: "22px"
  input:
    backgroundColor: "{colors.working-surface}"
    textColor: "{colors.primary-ink}"
    rounded: "{rounded.field}"
    height: "52px"
---

# Design System: Party Hub

## Overview

**Creative North Star: "Low-light confidence"**

Party Hub is built to remain obvious in a noisy, dim room. Its visual system is dark, flat, and operational: compact working areas, large transport controls, and one scarce signal color that identifies live playback state. Midnight Violet is the default; the DJ may choose another curated theme without changing layout or semantics. Cover artwork may be expressive; the surrounding chrome stays quiet.

The interface should feel like dependable equipment without imitating a mixer, terminal, or fictional control panel. Standard web controls remain recognizable, and repeated DJ actions respond immediately.

**Key Characteristics:**
- True-black environment with lightness-only layering
- One theme accent for live data, progress, and the primary queue action
- Large, legible controls with visible keyboard focus
- Cover artwork as content rather than decoration
- Restrained, purposeful motion

## Colors

Every theme uses a near-black ground, two surface steps, high-contrast text, and one accent. Midnight Violet is the default. Signal Orange preserves the original warmer equipment character, Electric Blue is cooler, Terminal Green recalls classic phosphor displays, Sneaky Yellow adds a playful low-key signal, Cherry Red brings deep after-hours warmth, and High Contrast maximizes edge and text separation. Theme choice is stored only on the DJ laptop; guest phones keep the guest palette.

**The Signal Scarcity Rule.** The selected accent marks progress, the current queue position, the primary song action, and live connection state. It does not decorate every navigation item, panel edge, or secondary action.

**The Semantic Stability Rule.** Positive, warning, and error colors keep their meaning in every theme. Theme accents never replace recovery colors.

**The Honest State Rule.** Warning and error colors appear only when those states exist. State meaning is also written in text and never depends on hue alone.

## Typography

Inter Variable is the single interface family. The system uses a compact app scale rather than display typography: section titles are concise, track titles may grow responsively, and queue metadata remains readable at distance. Updating times and durations use tabular figures.

**The Two-Line Rule.** Track titles in the primary deck stop at two visible lines. The complete value remains available through the queue and native title text.

## Layout

Desktop uses two working columns with one consistent gap: a compact now-playing card on the left and a right stack with a standalone Add your song card above the queue. The queue list owns desktop overflow; the document does not scroll. At tablet and phone widths the cards stack in the order player, song adder, queue. The page opens at the top and never scrolls itself to the current queue item.

Outer margins tighten from 28px to 16px on small screens. Interactive targets have a 44px minimum. The content stops expanding at a practical laptop width rather than stretching across very wide displays.

## Elevation & Depth

Depth comes from lightness steps only. Resting panels have no shadow or glow. Focus is expressed with a clear white outline; transient notices and selected rows use a stronger surface tone.

**The Lightness-Is-Depth Rule.** Do not combine borders and shadows on resting surfaces. A panel either changes tone or remains on the ground.

## Shapes

Large working panels use broad 20px corners. Rows and notices use medium 12px corners. Fields use 14px corners, while transport and primary controls use full circular or pill geometry. Artwork uses tighter corners so it reads as media inside the system.

## Components

### Buttons
- Primary actions are white on near-black and at least 44px high.
- Press feedback uses a subtle scale change; keyboard-triggered actions do not animate.
- Disabled controls stay visible but clearly inactive.

### Cards / Containers
- Panels are broad working surfaces, not stacks of nested decorative cards.
- Selected queue rows use a raised neutral surface and a written current-state label for assistive technology.

### Inputs / Fields
- The DJ song entry lives in its own Add your song card above the queue: heading, prominent Search YouTube action, an “or paste a link” divider, the link field/action group, then concise format guidance.
- Clear and clipboard affordances remain inside the field group; validation occupies a stable feedback row.
- Focus strengthens the field edge without glow.

### YouTube Search
- Search is a shared modal task opened from the Add your song card on both the DJ dashboard and joined guest view.
- The dialog owns the search field and explicit submit action. It shows loading, empty, error, retry, and per-result adding states without changing the queue layout underneath.
- Result rows use YouTube artwork as content, keep titles to one line, show channel and tabular duration, and retain a fixed trailing Add to queue action.
- Current and upcoming duplicates remain visible but disabled as “Already queued”; played history may be queued again.
- Escape and backdrop close the dialog, keyboard focus stays trapped while open, and focus returns to the Search YouTube trigger.

### Queue Spine
- A stable vertical line communicates order.
- Filled neutral markers mean played, an orange marker means current, and hollow markers mean queued.
- Reordering moves the item and its state together; color never carries the state alone.

### Playback Controls
- The main play/pause target is the largest control.
- Previous, next, seek, and volume controls remain visually subordinate but fully keyboard accessible.
- Progress exposes semantic values to assistive technology.

### Guest Contribution
- The guest route is phone-first, with one required-name join step followed by Search YouTube, direct-link entry, now playing, and the shared queue.
- Attribution appears as queue metadata. A guest-owned removable track gets one trailing trash action; all other rows stay read-only.
- QR sharing and YouTube search are the only elevated overlays. The QR field is functional media; guest search reuses the same focused modal and result rows as the DJ.
- Guest success, expired access, reconnecting, empty queue, and submission-limit states are written explicitly.

## Do's and Don'ts

### Do:
- **Do** make current playback, queue order, and recovery states understandable within seconds.
- **Do** preserve at least 44px targets for party-critical actions.
- **Do** keep frequently repeated interactions instant or under 300ms.
- **Do** keep theme changes immediate and preserve contrast at every theme.
- **Do** test long multilingual titles, missing art, empty queues, and disconnected playback.

### Don't:
- **Don't** add gradients, glass, glows, rainbow visualizers, or decorative equalizers.
- **Don't** spend the selected accent on ordinary chrome.
- **Don't** let theme selection alter permissions, queue behavior, or guest presentation.
- **Don't** introduce KPI cards, promotional hero layouts, or dashboard vanity metrics.
- **Don't** hide queue control behind hover-only behavior or animation.
