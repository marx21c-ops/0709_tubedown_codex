FROM golang:1.22 AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o server ./cmd/server

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends python3 python3-pip ffmpeg ca-certificates nodejs \
    && pip3 install --break-system-packages --no-cache-dir "yt-dlp[default,curl-cffi]" \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*
COPY --from=builder /app/server /server
EXPOSE 8080
CMD ["/server"]
