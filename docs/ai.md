# AI song picks

**Let AI pick** chooses a new YouTube song for the current vibe and inserts it next. The current song keeps playing. Use **Undo** to remove the suggestion before it starts.

## Connect your AI provider

1. Select **Let AI pick** for first-time setup, or the settings icon beside it.
2. Choose a provider and enter your API key. Ollama and compatible servers can work without a key when their server permits it.
3. Models load automatically after you enter a key or connect a local server. Choose a model or type its ID; use **Refresh models** to retry.
4. Select **Test connection**, then **Save**.
5. Select **Let AI pick** again to choose a song.

Supported connections: ChatGPT plan sign-in, OpenAI (Responses API), Anthropic (Messages API), Google Gemini, Ollama, and servers supporting OpenAI-compatible Chat Completions. Each DJ supplies their own account and model. See **Sign in with ChatGPT** below to connect a ChatGPT plan without an API key. No API key is bundled with the app.

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

The connection test makes one small generation request. A pick normally makes two model requests. Provider charges and rate limits depend on your account and model. Generation calls happen only when you select Test or Let AI pick; ordinary search and playback do not call the AI provider.

A pick uses a shared YouTube search slot and only one AI pick can run at a time. Failed or canceled requests do not insert a track. Picks also work with an empty queue, preparing the selected song without autoplay.

The public interactive demo uses a simulated sample-catalog picker. It does not configure providers or send AI requests.

## Sign in with ChatGPT

For the local app, choose **ChatGPT plan** instead of an API provider:

1. Select **Continue with ChatGPT** and approve access in the browser.
2. Return to Party Hub and acknowledge the first-use confirmation.
3. Models load automatically. Choose an available model, then **Save**.

No OpenAI API key is needed. Eligible requests use your ChatGPT plan or credits and its usage limits. **Manage usage** opens [ChatGPT usage settings](https://chatgpt.com/settings/usage). A connection test is optional for this connection type and sends one small request.

Account registrations stay separate, including registrations with the same email in different workspaces. Select a saved account or **Add a ChatGPT account** to connect another. A new sign-in does not replace your active provider/model until you save. The connected account is shown in AI settings, with a plan indicator beside the AI picker.

**Remember sign-in securely** stores access, refresh, and identity tokens in the OS credential store. Uncheck it for session-only sign-in. Account labels, registration IDs, and a stable opaque laptop ID are saved in `ai.json.chatgpt` beside `ai.json`; this file contains no OAuth tokens. Credentials never go into browser storage or the repository. Access is renewed automatically when needed.

**Sign out** stops in-flight picks for that account, attempts to revoke its renewable session, and clears its local credentials. It retains the account registration for a later sign-in. If remote revocation cannot be confirmed, disconnect Party Hub in ChatGPT Settings. Switching to another provider alone does not sign out saved ChatGPT accounts.

Declined sign-in, expired access, interrupted responses, and usage limits leave the queue unchanged. Party Hub never falls back automatically to an API key. The public demo continues to simulate picks without login or AI requests.

This integration follows OpenAI's [local open-source sign-in flow](https://developers.openai.com/siwc/token-sharing-open-source/sign-in) and [ChatGPT plan inference requirements](https://developers.openai.com/siwc/token-sharing-open-source/models-and-inference). Account eligibility and availability are controlled by OpenAI; this setup does not establish support for a remotely hosted Party Hub service.
