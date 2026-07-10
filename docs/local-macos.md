# TubeDown Local for macOS

TubeDown Local downloads directly from YouTube to this Mac and merges video/audio locally. Video bytes and browser cookies are not sent to Railway.

## Security boundaries

- Listens only on `127.0.0.1:8787`, never on the LAN.
- Rejects non-loopback clients and non-local Host/Origin values.
- Protects every API request with a random per-process token.
- Uses a restrictive Content Security Policy and disallows framing.
- Executes `yt-dlp` directly without a shell.
- Accepts only predefined quality selectors.
- Writes only to `~/Downloads/TubeDown`.
- Runs at most one download job at a time.
- Reads Chrome cookies locally through `yt-dlp`; cookies are never returned to the browser UI.

## Install

Required commands:

```sh
brew install yt-dlp ffmpeg deno
```

Install the local app:

```sh
./scripts/install-local.sh
```

The installer creates `~/Applications/TubeDown Local.app` and opens it. The app then opens `http://127.0.0.1:8787` in the default browser.

Downloads are saved in `~/Downloads/TubeDown`.

## Stop

Use Activity Monitor to quit `TubeDown`, or run:

```sh
pkill -x TubeDown
```
