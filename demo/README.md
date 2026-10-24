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

The build is in `demo/dist/`; preview uses http://127.0.0.1:4174. Relative asset URLs support deployment under a GitHub Pages project path. Guest preview uses `?view=guest`, so it needs no special server routing. The GitHub Pages workflow is included; enable Pages in the repository settings before the first deployment.

## How it stays separate

- All demo-specific files live in `demo/`, alongside the main app.
- Vite imports the existing `../src` UI and replaces its API module at build time with `src/api.ts`. Its legacy queue loader is also replaced so the demo never reads the real app’s browser queue.
- The main app's code, API behavior, scripts, and production build are unchanged.
- Existing dependencies are reused from the parent `node_modules`; this folder is not standalone yet.
- Demo source, tests, and artwork are tracked in Git; generated `dist/` and `node_modules/` stay ignored.

## Deploy to GitHub Pages

1. Open the repository’s **Settings → Pages** and set **Source** to **GitHub Actions**.
2. Commit and push the demo and `.github/workflows/deploy-demo.yml` to `main`.
3. Watch **Actions → Deploy demo to GitHub Pages** for the published URL.

The workflow installs the root dependencies with Yarn Classic 1.22.22, runs the demo tests, builds the demo, and publishes only `demo/dist/`. Future pushes to `main` redeploy automatically. You can also run it manually from the Actions tab; production deployments are limited to `main`.

The expected project URL is https://amitpdev.github.io/partyhub/. No Go service, ngrok tunnel, or separate hosting account is needed.
