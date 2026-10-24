# Party Hub interactive demo

A local-only, browser-based demo that reuses the real dashboard, themes, guest UI, and playlist dialogs. Sample songs and original SVG artwork are bundled locally. Playback is simulated; no audio is fetched or played. No Go service, yt-dlp, mpv, or ngrok is used.

## Run

From the main Party Hub folder, install its existing dependencies once with `yarn install`, then:

```sh
yarn --cwd demo dev
```

Open http://127.0.0.1:5174. Use **DJ view** and **Guest view** to explore both sides of the party. Search any artist or song: matching sample titles are filtered, and unmatched searches show six sample songs so you can still try adding to the queue. The search dialog explains that the full app searches YouTube. Add a song as a guest, then switch back to the DJ to see it. The demo starts paused.

Use **Reset** to restore the starting queue, playlists, guest access, and purple theme. Demo queue/library changes use the separate `partyhub.demo.v1` browser storage key. Real Party Hub queue storage is never read or written by the demo API. Each browser has its own demo; it is not a shared online party.

## Build and check

```sh
yarn --cwd demo test
yarn --cwd demo build
yarn --cwd demo preview
```

The build is in `demo/dist/`; preview uses http://127.0.0.1:4174. Relative asset URLs support deployment under a GitHub Pages project path. Guest preview uses `?view=guest`, so it needs no special server routing. No hosting or GitHub repository is configured yet.

## How it stays separate

- All demo-specific files live in `demo/`, alongside the main app.
- Vite imports the existing `../src` UI and replaces its API module at build time with `src/api.ts`. Its legacy queue loader is also replaced so the demo never reads the real app’s browser queue.
- The main app's code, API behavior, scripts, and production build are unchanged.
- Existing dependencies are reused from the parent `node_modules`; this folder is not standalone yet.
- Demo source, tests, and artwork are tracked in Git; generated `dist/` and `node_modules/` stay ignored.

For GitHub Pages deployment from this repository, install dependencies at the repository root, run `yarn --cwd demo build`, and publish the `demo/dist/` artifact. No deployment workflow is configured yet.
