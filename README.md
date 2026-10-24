# Party Hub

🎧 Turn your laptop into the party’s DJ booth, where you can play YouTube tracks, save your favorite sets, and let friends scan a QR code to add songs while the party is going.

Audio plays on the laptop connected to your speakers, and you keep the power to skip.

<img width="600" alt="0c47e207-aeb8-4fa5-be98-f1b62b24ae9f" src="https://github.com/user-attachments/assets/d8061e7d-c391-4151-b57d-5c1d5c828107" />

## Features

- Search YouTube without an API key
- Play audio on the host laptop, typically without ads
- Persist the queue and volume between restarts
- Save, import, export, and share local playlists
- Let friends add songs through optional QR-code guest access
- Limit guests to managing their own queued tracks

## Requirements

- Node.js 22 or newer
- Yarn Classic 1.22.22
- Go 1.27.3 or newer
- [`yt-dlp`](https://github.com/yt-dlp/yt-dlp)
- [`mpv`](https://mpv.io/)
- [`ngrok`](https://ngrok.com/) for guest access

On macOS:

```sh
npm install --global yarn@1.22.22
brew install yt-dlp mpv ngrok
```

## Run a party

```sh
yarn install
ngrok config add-authtoken YOUR_TOKEN
yarn party
```

Open [http://127.0.0.1:8787](http://127.0.0.1:8787). Party Hub builds the app, starts the local player, and opens an ngrok tunnel. Press `Ctrl+C` to stop it.

## Invite friends

Select **Invite friends** to display a QR code. Guests enter a name, then search YouTube or paste a link. They can keep up to three upcoming tracks and remove only their own.

Ending guest access invalidates existing sessions without stopping playback or the tunnel. Use **Allow guests** to create a fresh invite. Anyone with the active link can join, so end sharing when the party is over.

## Local development

```sh
yarn dev
```

Open [http://127.0.0.1:5173](http://127.0.0.1:5173). This starts Vite and the Go player without ngrok.

```sh
yarn dev:web     # Frontend only
yarn dev:server  # Go service only
yarn verify      # Lint, test, and build
```

## Keyboard controls

- `Space`: play or pause
- `N`: next track
- `B`: previous track

## More documentation

- [Playlist guide](docs/playlists.md)
- [Architecture and YouTube playback](docs/architecture.md)

## Current limitations

- No visible video or karaoke output
- No Spotify support
- Playback does not resume automatically after restarting the Go service
- Guest identity is party-scoped and does not use accounts
- Guest access requires the ngrok CLI

## License

Party Hub is licensed under the [GNU Affero General Public License v3.0](LICENSE). You may use, modify, and self-host it, subject to the license terms.

Copyright © 2026 Amit Palomo.
