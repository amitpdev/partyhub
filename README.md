# Party Hub

Party Hub is a host-laptop dashboard for running music at parties and gatherings. Search YouTube or paste a link, arrange the live set, and let friends add songs from their phones while audio stays on the laptop connected to the speakers.

<img width="600" alt="0c47e207-aeb8-4fa5-be98-f1b62b24ae9f" src="https://github.com/user-attachments/assets/d8061e7d-c391-4151-b57d-5c1d5c828107" />

## Phase 1

- YouTube tracks only
- DJ and guest YouTube search without an API key
- Audio-only playback on the host laptop
- Add, play, reorder, skip, and remove queued tracks
- Pause, seek, and control volume
- Automatic advance and unavailable-track skipping
- Server-backed queue and volume persistence
- Optional QR-code guest access through ngrok
- Guest names, attribution, limits, and ownership-based removal

Search returns up to 12 standard videos and excludes live or upcoming streams, Shorts, videos with unknown duration, and videos longer than 15 minutes. Selecting a result uses the same queue validation and insertion path as a pasted link. Guests can search after joining an active shared party; queue limits apply when they add a result, not while they search.

## Requirements

- Node.js 22 or newer
- Go 1.24 or newer
- [`yt-dlp`](https://github.com/yt-dlp/yt-dlp)
- [`mpv`](https://mpv.io/)
- [`ngrok`](https://ngrok.com/) for guest access

On macOS:

```sh
brew install yt-dlp mpv ngrok
```

## Run locally

```sh
yarn install
yarn dev
```

Open [http://127.0.0.1:5173](http://127.0.0.1:5173). The command starts both the Vite dashboard and the Go playback helper bound to localhost.

The Go service owns the queue and persists it under the operating system's user configuration directory. On its first empty start, the dashboard imports a queue saved by the earlier browser-only version.

Useful separate commands:

```sh
yarn dev:web
yarn dev:server
yarn verify
```

## Share a party with guests

Sign in to ngrok once before the first party:

```sh
ngrok config add-authtoken YOUR_TOKEN
```

Stop `yarn dev`, then run:

```sh
yarn party
```

This command:

1. builds the dashboard;
2. starts the local Go player at `http://127.0.0.1:8787`;
3. starts one ngrok HTTPS tunnel;
4. registers the temporary public URL; and
5. keeps both processes running until you press `Ctrl+C`.

Open the local DJ dashboard and select **Guests can join** to display the QR code. The QR link contains a temporary guest capability. Ending guest access invalidates existing guest sessions but keeps the queue.

Guests must enter a display name. They can:

- search YouTube or paste a link for individual videos up to 15 minutes;
- keep at most three upcoming tracks in the queue;
- see who added each track; and
- remove only their own upcoming tracks.

Guests cannot control playback, reorder tracks, remove other people's tracks, or clear the queue. Duplicate videos, playlists, and live streams are rejected.

The free ngrok URL changes between sessions and may show ngrok's one-time browser warning before the Party Hub join screen. Anyone holding the active QR link can request a guest session, so end sharing when the party is over.

## Local playlists

Open **Playlist** in the top bar to browse saved playlists. Each playlist appears once, with its name, track count, and duration. Select it to replace the queue after confirmation. Playback stops and the first song is prepared without autoplay; volume stays unchanged.

- **Load** adds playlists from a JSON file to the library as fresh copies (8 MB maximum). Select the imported playlist to use it.
- The **…** button beside each playlist contains **Rename**, **Export**, **Share**, and **Delete**. Export downloads that playlist only. Share opens the system share sheet for the playlist file when supported; otherwise it downloads the file for you to send.
- The separate **Save** icon in the top bar saves the entire current queue (played, current, and upcoming songs). For a loaded playlist, choose **Save changes** or **Save as new**.
- The **Save** icon lights up when the queue is unsaved or the loaded playlist has changes. Playing, skipping, seeking, and changing volume do not mark it as changed.
- Loading another playlist while there are unsaved playlist changes offers save, discard, or cancel.
- The live queue still saves automatically, independently of named playlists. The playlist connection and change indicator survive service restarts.

The Go service stores versioned `playlists.json` alongside its `state.json` in the operating system's user configuration directory (or alongside `PARTYHUB_STATE_PATH` when set). Playlist controls are host-only. Saving and browsing do not require mpv or a YouTube connection; song availability is handled at playback time. Duplicate names are allowed. Loading skips repeated videos and rejects sets exceeding the existing queue capacity without partially changing the queue.

## Keyboard controls

- `Space`: play or pause
- `N`: next track
- `B`: previous track

## Playback architecture

The localhost-only Go service owns and persists the shared queue. It:

1. validates and normalizes YouTube URLs;
2. uses `yt-dlp` to search YouTube and resolve metadata without an API key;
3. starts one audio-only `mpv` process;
4. controls it through mpv's Unix-socket IPC; and
5. synchronizes queue and playback state to every browser with server-sent events.

Automatic advancement also runs in the Go service, preventing multiple browsers from racing the player. Browser input is validated before becoming a process argument. The service remains bound to `127.0.0.1`; ngrok reaches it through its local agent.

Guest searches are unlimited over time. To protect the host laptop, a newer search replaces an older search from the same guest and no more than three `yt-dlp` searches run simultaneously.

## YouTube and advertisements

Party Hub deliberately does not use the YouTube IFrame Player. It follows the direct-stream `yt-dlp`/`mpv` approach proven in the local BoardTube project, which did not play YouTube-served advertisements during development and smoke testing.

This is not a permanent guarantee. It is an unofficial integration, can break when YouTube changes delivery, and may conflict with YouTube's terms and developer policies. Keep `yt-dlp` current and use the project with an understanding of that tradeoff.

## Current limitations

- No visible video or karaoke output
- No Spotify support
- Restoring the page restores queue state but does not autoplay
- Guest identity is party-scoped and does not use accounts
- Guest access currently depends on a separately installed ngrok CLI
- The smoke test verifies direct muted media playback, not every possible advertisement or regional playback path
