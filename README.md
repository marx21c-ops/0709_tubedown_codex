# downloader-2607

Go/Fiber API server for extracting video metadata and streaming downloads through `yt-dlp`.

This service is intended for personal archiving and backing up content you own or are authorized to download. It does not bypass DRM.

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
| `RATE_LIMIT_PER_IP` | `2` | concurrent downloads per IP |

## Deploy

The included `Dockerfile` installs `yt-dlp` and `ffmpeg` into a slim Debian runtime image. On Railway, use Dockerfile-based deployment and set `PORT` if needed.
