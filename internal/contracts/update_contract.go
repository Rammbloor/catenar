package contracts

type UpdateCheckResult struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion"`
	UpdateAvailable bool   `json:"updateAvailable"`
	ReleaseURL      string `json:"releaseUrl,omitempty"`
	DownloadURL     string `json:"downloadUrl,omitempty"`
	DownloadName    string `json:"downloadName,omitempty"`
	PublishedAt     string `json:"publishedAt,omitempty"`
}

type UpdateCheckResponse struct {
	Ok    bool               `json:"ok"`
	Data  *UpdateCheckResult `json:"data,omitempty"`
	Error *ErrorEnvelope     `json:"error,omitempty"`
}
