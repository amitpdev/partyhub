# Playlist guide

Open **Playlist** in the top bar to browse saved playlists. Select a playlist to replace the queue after confirmation. Playback stops, the first song is prepared without autoplay, and the volume stays unchanged.

## Managing playlists

- **Load** imports playlists from a JSON file as fresh copies, up to 8 MB.
- The **…** menu contains **Rename**, **Export**, **Share**, and **Delete**.
- **Share** opens the system share sheet when supported and otherwise downloads the file.
- The top-bar **Save** button stores the entire current queue, including played, current, and upcoming songs.
- When editing a loaded playlist, choose **Save changes** or **Save as new**.
- Loading another playlist with unsaved changes offers save, discard, or cancel.

The Save button lights up when the queue is unsaved or the loaded playlist has changed. Playback actions such as playing, skipping, seeking, and changing volume do not count as playlist changes.

## Storage and behavior

The live queue is saved independently of named playlists. Playlist selection and change status survive service restarts.

The Go service stores versioned `playlists.json` beside `state.json` in the operating system's user configuration directory. When `PARTYHUB_STATE_PATH` is set, it uses that file's directory instead.

Playlist controls are host-only. Saving and browsing do not require mpv or a YouTube connection. Duplicate names are allowed. Loading skips repeated videos and rejects playlists that exceed queue capacity without partially changing the queue.
