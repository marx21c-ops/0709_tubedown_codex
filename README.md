# downloader-2607

Go/Fiber API server for extracting video metadata and streaming downloads through `yt-dlp`.

This service is intended for personal archiving and backing up content you own or are authorized to download. It does not bypass DRM.

Initial allowlist: YouTube, TikTok, Twitter/X, Instagram, Reddit, Twitch, SoundCloud, Naver TV, Bilibili, Facebook, Vimeo.

## Requirements

- Go 1.22+
- `yt-dlp`
- `ffmpeg`

## Run locally

```sh
go mod tidy
go run ./cmd/server
```

The server listens on `:8080` by default.

## API

### Metadata

```sh
curl -X POST http://localhost:8080/api/v1/metadata \
  -H 'content-type: application/json' \
  -d '{"url":"https://www.youtube.com/watch?v=..."}'
```

### Download

```sh
curl -L 'http://localhost:8080/api/v1/download?url=https%3A%2F%2Fwww.youtube.com%2Fwatch%3Fv%3D...&format_id=18' \
  -o video.mp4
```

## Environment

| Name | Default | Description |
|---|---:|---|
| `PORT` | `8080` | HTTP server port |
| `YTDLP_BINARY` | `yt-dlp` | yt-dlp executable path |
| `METADATA_TIMEOUT` | `30s` | metadata extraction timeout |
| `DOWNLOAD_TIMEOUT` | `30m` | streaming download timeout |
| `DOWNLOAD_WORKERS` | `1` | maximum concurrent yt-dlp download/merge jobs |
| `RATE_LIMIT_PER_IP` | `2` | concurrent downloads per IP |
| `YTDLP_JS_RUNTIME` | `deno` | JavaScript runtime passed to yt-dlp |
| `YTDLP_IMPERSONATE` | `chrome` | browser impersonation target for yt-dlp/curl_cffi |
| `YTDLP_PROXY` | empty | optional proxy URL for yt-dlp |
| `YTDLP_COOKIES_FILE` | empty | optional Netscape cookies file path |
| `YTDLP_COOKIES_CONTENT` | empty | optional Netscape cookies.txt content written to `/tmp/yt-dlp-cookies.txt` at startup |
| `YTDLP_COOKIES_CONTENT_FILE` | `/tmp/yt-dlp-cookies.txt` | target path for `YTDLP_COOKIES_CONTENT` |

YouTube may block datacenter IPs such as Railway with `HTTP 429` or bot checks. For production YouTube use, configure `YTDLP_PROXY` and, when needed, mount/provide a cookies file and set `YTDLP_COOKIES_FILE`.

For Railway, prefer storing cookies in `YTDLP_COOKIES_CONTENT` instead of committing a cookies file. Export cookies in Netscape `cookies.txt` format and paste the full file content into the Railway variable.

## Deploy

The included `Dockerfile` installs `yt-dlp` and `ffmpeg` into a slim Debian runtime image. On Railway, use Dockerfile-based deployment and set `PORT` if needed.
