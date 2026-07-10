package model

type MetadataRequest struct {
	URL string `json:"url"`
}

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Format struct {
	FormatID   string `json:"format_id"`
	Resolution string `json:"resolution"`
	Ext        string `json:"ext"`
	Note       string `json:"note,omitempty"`
	Protocol   string `json:"protocol,omitempty"`
	Quality    int    `json:"quality,omitempty"`
}

type MetadataResponse struct {
	Title     string   `json:"title"`
	Thumbnail string   `json:"thumbnail"`
	Duration  float64  `json:"duration"`
	Formats   []Format `json:"formats"`
}
