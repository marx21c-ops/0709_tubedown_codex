package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"downloader-2607/internal/localapp"
	"downloader-2607/internal/service"
)

const address = "127.0.0.1:8787"

func main() {
	os.Setenv("PATH", "/opt/homebrew/bin:/usr/local/bin:"+os.Getenv("PATH"))
	for _, binary := range []string{"yt-dlp", "ffmpeg", "deno"} {
		if _, err := exec.LookPath(binary); err != nil {
			fmt.Fprintf(os.Stderr, "%s가 필요합니다. Homebrew로 설치한 후 다시 실행하세요.\n", binary)
			os.Exit(1)
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "사용자 폴더를 확인할 수 없습니다.")
		os.Exit(1)
	}
	downloadDir := filepath.Join(home, "Downloads", "TubeDown")
	ytdlp := service.NewYTDLP(service.Config{
		Binary:          "yt-dlp",
		MetadataTimeout: 90 * time.Second,
		DownloadTimeout: 12 * time.Hour,
		DownloadWorkers: 1,
		CookiesBrowser:  "chrome",
		JSRuntime:       "deno",
	})
	app, err := localapp.New(ytdlp, downloadDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	url := "http://" + address
	fmt.Println("TubeDown Local:", url)
	fmt.Println("저장 위치:", downloadDir)
	go func() {
		time.Sleep(500 * time.Millisecond)
		_ = exec.Command("open", url).Start()
	}()
	if err := app.Listen(address); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
