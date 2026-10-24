# Architecture and YouTube playback

## Playback service

The localhost-only Go service owns and persists the shared queue. It:

1. validates and normalizes YouTube URLs;
2. uses `yt-dlp` to search YouTube and resolve metadata without an API key;
3. starts one audio-only `mpv` process;
4. controls it through mpv's Unix-socket IPC; and
5. synchronizes queue and playback state through server-sent events.

Automatic advancement runs in the Go service, preventing multiple browsers from racing the player. Browser input is validated before becoming a process argument. The service remains bound to `127.0.0.1`; ngrok reaches it through its local agent.

The queue and volume persist in the operating system's user configuration directory. On its first empty start, the dashboard imports a queue saved by the earlier browser-only version.

## Search and validation

Search returns up to 24 standard videos. It excludes live and upcoming streams, Shorts, videos with unknown duration, and videos longer than 15 minutes. Search results and pasted links use the same queue validation path.

Guest searches are unlimited over time. A newer search replaces an older search from the same guest, and no more than three `yt-dlp` searches run simultaneously.

## YouTube playback

Party Hub plays YouTube audio through `yt-dlp` and `mpv` rather than the YouTube IFrame Player. Ad-free playback is not guaranteed.

This unofficial integration can break when YouTube changes delivery and may conflict with YouTube's terms and developer policies. Keep `yt-dlp` current.

## Development while a party is running

Keep `yarn party` running and start `yarn dev:web` in another terminal. Use the dashboard on port 5173 for automatic frontend updates while the player and tunnel continue running.

Run `yarn build` and refresh guest browsers to serve updated frontend assets. Refreshing the dashboard does not interrupt playback. If the Go service stops, the dashboard retries every three seconds and reconnects when it returns without starting playback.
