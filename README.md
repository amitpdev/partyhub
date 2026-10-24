# Party Hub

🎧 Turn your laptop into the party’s DJ booth, where you can play YouTube tracks, save your favorite sets, and let friends scan a QR code to add songs while the party is going.

Audio plays on the laptop connected to your speakers, and you keep the power to skip.

<img width="600" alt="0c47e207-aeb8-4fa5-be98-f1b62b24ae9f" src="https://github.com/user-attachments/assets/d8061e7d-c391-4151-b57d-5c1d5c828107" />

## Features

- YouTube tracks only
- DJ and guest YouTube search without an API key
- Audio-only playback on the host laptop
- Add, play, reorder, skip, and remove queued tracks
- Pause, seek, and control volume
- Automatic advance and unavailable-track skipping
- Server-backed queue and volume persistence
- Saved local playlists with load, save, rename, and delete controls
- Playlist file import, export, and sharing
- Unsaved playlist indicators and save prompts when switching sets
- Optional QR-code guest access through ngrok
- Guest names, attribution, limits, and ownership-based removal

Search returns up to 24 standard videos and excludes live or upcoming streams, Shorts, videos with unknown duration, and videos longer than 15 minutes. Selecting a result uses the same queue validation and insertion path as a pasted link. Guests can search after joining an active shared party; queue limits apply when they add a result, not while they search.

## Requirements

- Node.js 22 or newer
- Yarn Classic 1.22.22 (the version declared in `package.json`)
- Go 1.27.3 or newer (as declared in `go.mod`)
- [`yt-dlp`](https://github.com/yt-dlp/yt-dlp)
- [`mpv`](https://mpv.io/)
- [`ngrok`](https://ngrok.com/) for guest access

Install Yarn Classic after installing Node.js:

```sh
npm install --global yarn@1.22.22
```

On macOS, install the playback tools and ngrok with Homebrew:

```sh
brew install yt-dlp mpv ngrok
```

## Run a party

Install dependencies and configure ngrok once:

```sh
yarn install
ngrok config add-authtoken YOUR_TOKEN
```

Start Party Hub:

```sh
yarn party
```

Open the DJ dashboard at [http://127.0.0.1:8787](http://127.0.0.1:8787). This builds the web app, starts the local Go player, and opens an ngrok tunnel so friends can join from their phones. Both processes keep running until you press `Ctrl+C`.

The Go service owns the queue and persists it under the operating system's user configuration directory. On its first empty start, the dashboard imports a queue saved by the earlier browser-only version.

## Invite friends

Select **Invite friends** in the local DJ dashboard to display the QR code. The QR link contains a temporary guest capability. Ending guest access invalidates existing guest sessions but keeps the queue, playback, and running ngrok tunnel. Select **Allow guests** in the same dialog to re-enable access without restarting anything; friends must scan the new QR code.

Guests must enter a display name. They can:

- search YouTube or paste a link for individual videos up to 15 minutes;
- keep at most three upcoming tracks in the queue;
- see who added each track; and
- remove only their own upcoming tracks.

Guests cannot control playback, reorder tracks, remove other people's tracks, or clear the queue. Duplicate videos, playlists, and live streams are rejected.

Free ngrok accounts include a [stable, automatically assigned dev domain](https://ngrok.com/blog/free-static-domains-ngrok-users). Guests may see ngrok's browser warning before the Party Hub join screen. Party Hub creates a fresh guest token when a party starts, so use the new QR code each session even if the domain stays the same. Anyone holding the active QR link can join, so end sharing when the party is over.

## Local development

For development without guest access, run:

```sh
yarn dev
```

Open [http://127.0.0.1:5173](http://127.0.0.1:5173). This starts Vite with automatic frontend updates and the local Go player, without starting ngrok. Stop this command before running `yarn party` because both start the Go service on port 8787.

To edit the frontend while a party is running, keep `yarn party` running and start `yarn dev:web` in another terminal. Use the dashboard on port 5173 for automatic updates; the player and ngrok keep running. Run `yarn build` and refresh guest browsers when you want them to receive the updated frontend. Refreshing the dashboard does not interrupt playback; the Go service keeps the player running. If the service stops, the dashboard retries every three seconds and reconnects automatically when it returns, without refreshing the page or starting playback.

Useful separate commands:

```sh
yarn dev:web
yarn dev:server
yarn verify
```

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

## YouTube playback

Party Hub plays YouTube audio through `yt-dlp` and `mpv` rather than the YouTube IFrame Player. Ad-free playback is not guaranteed.

This unofficial integration can break when YouTube changes delivery and may conflict with YouTube's terms and developer policies. Keep `yt-dlp` current.

## Current limitations

- No visible video or karaoke output
- No Spotify support
- Restarting the Go service restores the saved queue without automatically resuming playback
- Guest identity is party-scoped and does not use accounts
- Guest access currently depends on a separately installed ngrok CLI
