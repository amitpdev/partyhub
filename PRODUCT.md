# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Stack

React, TypeScript, and Vite for the dashboard; a local Go service controls `yt-dlp` and `mpv`.

## Users

The primary user is the host or designated DJ at a party, friends gathering, or karaoke session. They operate a laptop connected to the speakers, often from a few feet away in a dim room. Party guests are secondary users who join from a phone to contribute songs without gaining playback control.

## Product Purpose

Party Hub makes a shared music queue easy to operate without Spotify's dense general-purpose interface. Success means the DJ can see what is playing, understand what is next, and control or change the queue immediately.

## Positioning

Party Hub is a purpose-built party control surface: large, legible queue controls plus shared YouTube search and playback from the host laptop, with no account or API key required.

## Operating Context

The player runs locally on one host laptop connected to speakers. The DJ controls playback there. Guests may scan a temporary ngrok-backed QR code to join a phone-first contribution view. The interface prioritizes queue control in dim-room conditions and must remain clear at a distance.

## Capabilities and Constraints

- The local DJ and joined guests can search YouTube or paste a direct link; playback control remains on the host laptop.
- Search requires no YouTube API key, returns up to 12 standard videos, and excludes live or upcoming streams, Shorts, unknown durations, and videos longer than 15 minutes.
- The Go service owns queue order, current item, and volume, persists them locally, and synchronizes browsers over server-sent events.
- Guests join with a required display name, search without a usage quota, add directly in FIFO order, and can remove only their own upcoming tracks.
- Guest submissions are attributed, limited to three upcoming tracks, deduplicated, and restricted to non-live individual videos up to 15 minutes.
- Search concurrency is bounded on the host: a newer request replaces the same viewer's older search, and at most three searches run simultaneously.
- Playback, reordering, global removal, and queue clearing remain DJ-only.
- Playback uses a local `yt-dlp`/`mpv` helper. This avoids YouTube-served ads in observed use but is unofficial, can break when YouTube changes, and is not a permanent guarantee.
- Spotify, accounts, co-hosts, voting, and visible video are later phases.
- YouTube search results enter through the same normalized track and queue insertion path as pasted links.

## Brand Commitments

The product name is Party Hub. Its voice is direct, calm, and operational rather than promotional.

## Evidence on Hand

- A local prototype demonstrates the `yt-dlp`/`mpv` search and playback process-control approach.
- No logo, visual identity, customer proof, or production analytics exist yet.

## Product Principles

- Queue control is always the first and clearest job.
- Frequent actions are large, fast, and understandable at a glance.
- Playback failures never strand the party; explain and recover.
- Product state is honest: loading, unavailable, disconnected, empty, and restored states are explicit.
- Future discovery sources reuse the same normalized track and queue model.

## Accessibility & Inclusion

Use strong contrast, visible keyboard focus, large targets, reduced-motion support, and labels that do not rely on color or icons alone.
