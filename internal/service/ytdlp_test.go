package service

import "testing"

func TestDownloadSelector(t *testing.T) {
	tests := []struct {
		formatID string
		want     string
		merged   bool
	}{
		{"quality-1080", "bestvideo[height<=1080]+bestaudio/best[height<=1080]", true},
		{"quality-720", "bestvideo[height<=720]+bestaudio/best[height<=720]", true},
		{"18", "18", false},
	}

	for _, test := range tests {
		got, merged := downloadSelector(test.formatID)
		if got != test.want || merged != test.merged {
			t.Fatalf("downloadSelector(%q) = %q, %v; want %q, %v", test.formatID, got, merged, test.want, test.merged)
		}
	}
}

func TestMetadataResponseOffersMergedQualities(t *testing.T) {
	metadata := metadataJSON{
		Title: "test",
		Formats: []formatJSON{
			{FormatID: "137", Ext: "mp4", VCodec: "avc1", ACodec: "none", Height: 1080},
			{FormatID: "140", Ext: "m4a", VCodec: "none", ACodec: "mp4a"},
		},
	}

	response := metadata.toResponse()
	if len(response.Formats) != 4 {
		t.Fatalf("got %d quality presets, want 4", len(response.Formats))
	}
	if response.Formats[0].FormatID != "quality-1080" {
		t.Fatalf("highest preset = %q, want quality-1080", response.Formats[0].FormatID)
	}
	for _, format := range response.Formats {
		if format.Ext != "mp4" || format.Note != "video + audio" {
			t.Fatalf("preset %+v is not a merged MP4", format)
		}
	}
}
