# AI song picks

**Let AI pick** chooses a new YouTube song for the current vibe and inserts it next. The current song keeps playing. Use **Undo** to remove the suggestion before it starts.

## Connect your AI provider

1. Select **Let AI pick** for first-time setup, or the settings icon beside it.
2. Choose a provider and enter your API key. Ollama and compatible servers can work without a key when their server permits it.
3. Select **Load models**, or type your text-generation model ID.
4. Select **Test connection**, then **Save**.
5. Select **Let AI pick** again to choose a song.

Supported providers: OpenAI (Responses API), Anthropic (Messages API), Google Gemini, Ollama, and servers supporting OpenAI-compatible Chat Completions. Each DJ supplies their own account and model. No API key is bundled with the app.

For Ollama, start the local server and install a chat model first. The default server URL is `http://127.0.0.1:11434`. For compatible servers, enter the API base URL, usually ending in `/v1`. Remote servers require HTTPS; HTTP is allowed for loopback addresses only.

## Keys and local settings

- **Remember key securely** stores the key in macOS Keychain or, on Linux, the Secret Service keyring through `secret-tool`. Your OS may ask you to allow credential access.
- Uncheck it for a session-only key held in the Go service's memory. Enter it again after restarting the service.
- If secure storage is unavailable, use session-only mode. There is no plaintext key-file fallback.
- Non-secret provider/model preferences and an opaque credential ID are stored in `ai.json` beside the queue state file in the user configuration directory. Keys do not go into that file, browser storage, the repository, or guest responses.
- Keys are bound to the selected provider and server. Changing the destination does not forward a saved key to the new server.
- **Disconnect AI** removes the configured credential and resets the connection.

## What happens during a pick

Party Hub sends the recent song titles and artists to your provider to plan a search. It searches YouTube, excludes songs already in the set, and asks the model to choose from the actual results. The selected video is validated again before insertion. Guest names, session tokens, and resolved audio URLs are not sent to the model.

The connection test makes one small generation request. A pick normally makes two model requests. Provider charges and rate limits depend on your account and model. Calls happen only when you select Test or Let AI pick; ordinary search and playback do not call the AI provider.

A pick uses a shared YouTube search slot and only one AI pick can run at a time. Failed or canceled requests do not insert a track. Picks also work with an empty queue, preparing the selected song without autoplay.

The public interactive demo uses a simulated sample-catalog picker. It does not configure providers or send AI requests.
