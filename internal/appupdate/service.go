package appupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"catenar/internal/contracts"
)

const defaultGitHubAPIBaseURL = "https://api.github.com"

type Options struct {
	AppVersion     string
	GitHubRepo     string
	APIBaseURL     string
	HTTPClient     *http.Client
	RequestTimeout time.Duration
	Platform       string
	Architecture   string
}

type Service struct {
	appVersion     string
	githubRepo     string
	apiBaseURL     string
	httpClient     *http.Client
	requestTimeout time.Duration
	platform       string
	architecture   string
}

type githubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type githubReleaseResponse struct {
	TagName     string               `json:"tag_name"`
	Name        string               `json:"name"`
	HTMLURL     string               `json:"html_url"`
	PublishedAt string               `json:"published_at"`
	Draft       bool                 `json:"draft"`
	Prerelease  bool                 `json:"prerelease"`
	Assets      []githubReleaseAsset `json:"assets"`
}

func NewService(options Options) *Service {
	apiBaseURL := strings.TrimRight(options.APIBaseURL, "/")
	if apiBaseURL == "" {
		apiBaseURL = defaultGitHubAPIBaseURL
	}

	httpClient := options.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	requestTimeout := options.RequestTimeout
	if requestTimeout <= 0 {
		requestTimeout = 8 * time.Second
	}

	appVersion := strings.TrimSpace(options.AppVersion)
	if appVersion == "" {
		appVersion = "0.0.0"
	}

	return &Service{
		appVersion:     appVersion,
		githubRepo:     strings.TrimSpace(options.GitHubRepo),
		apiBaseURL:     apiBaseURL,
		httpClient:     httpClient,
		requestTimeout: requestTimeout,
		platform:       fallbackValue(options.Platform, runtime.GOOS),
		architecture:   fallbackValue(options.Architecture, runtime.GOARCH),
	}
}

func (s *Service) CurrentVersion() string {
	return s.appVersion
}

func (s *Service) Check(ctx context.Context) contracts.UpdateCheckResponse {
	if s.githubRepo == "" {
		return updateFailure(
			"application.update_repo_missing",
			"GitHub release repository is not configured.",
			nil,
		)
	}

	requestCtx, cancel := context.WithTimeout(ctx, s.requestTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(
		requestCtx,
		http.MethodGet,
		fmt.Sprintf("%s/repos/%s/releases/latest", s.apiBaseURL, s.githubRepo),
		nil,
	)
	if err != nil {
		return updateFailure("application.update_request_invalid", "The update check request could not be created.", nil)
	}

	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "Catenar")

	response, err := s.httpClient.Do(request)
	if err != nil {
		return updateFailure("application.update_unreachable", "Could not reach the update service.", nil)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return updateFailure(
			"application.update_unavailable",
			"Could not read the latest release information.",
			map[string]string{"status": response.Status},
		)
	}

	var release githubReleaseResponse
	if err := json.NewDecoder(response.Body).Decode(&release); err != nil {
		return updateFailure("application.update_decode_failed", "The update service returned an unreadable response.", nil)
	}

	latestVersion := normalizeVersion(release.TagName)
	if latestVersion == "" {
		latestVersion = normalizeVersion(release.Name)
	}
	if latestVersion == "" {
		return updateFailure("application.update_version_missing", "The latest release does not include a version tag.", nil)
	}

	result := contracts.UpdateCheckResult{
		CurrentVersion:  normalizeVersion(s.appVersion),
		LatestVersion:   latestVersion,
		UpdateAvailable: isVersionNewer(latestVersion, s.appVersion),
		ReleaseURL:      release.HTMLURL,
		PublishedAt:     release.PublishedAt,
	}
	if result.UpdateAvailable {
		if asset, ok := releaseAssetForPlatform(release.Assets, latestVersion, s.platform, s.architecture); ok {
			result.DownloadName = asset.Name
			result.DownloadURL = asset.BrowserDownloadURL
		}
	}

	return contracts.UpdateCheckResponse{
		Ok:   true,
		Data: &result,
	}
}

func fallbackValue(value string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func releaseAssetForPlatform(assets []githubReleaseAsset, version string, platform string, architecture string) (githubReleaseAsset, bool) {
	suffix := ""
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "darwin":
		suffix = "-darwin-universal.zip"
	case "windows":
		if strings.ToLower(strings.TrimSpace(architecture)) == "amd64" {
			suffix = "-windows-amd64-installer.exe"
		}
	case "linux":
		if strings.ToLower(strings.TrimSpace(architecture)) == "amd64" {
			suffix = "-linux-amd64"
		}
	}

	if suffix == "" {
		return githubReleaseAsset{}, false
	}

	expectedName := fmt.Sprintf("catenar-%s%s", normalizeVersion(version), suffix)
	for _, asset := range assets {
		if asset.Name == expectedName && strings.TrimSpace(asset.BrowserDownloadURL) != "" {
			return asset, true
		}
	}

	return githubReleaseAsset{}, false
}

func updateFailure(code, message string, details map[string]string) contracts.UpdateCheckResponse {
	return contracts.UpdateCheckResponse{
		Ok: false,
		Error: &contracts.ErrorEnvelope{
			Code:     code,
			Category: contracts.ErrorCategoryApplication,
			Message:  message,
			Details:  details,
		},
	}
}

func normalizeVersion(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimPrefix(trimmed, "v")
	trimmed = strings.TrimPrefix(trimmed, "V")
	return trimmed
}

func isVersionNewer(candidate string, current string) bool {
	candidateParts, candidateOK := parseVersionParts(candidate)
	currentParts, currentOK := parseVersionParts(current)
	if !candidateOK || !currentOK {
		return normalizeVersion(candidate) > normalizeVersion(current)
	}

	maxLength := max(len(candidateParts), len(currentParts))
	for index := 0; index < maxLength; index += 1 {
		candidatePart := versionPart(candidateParts, index)
		currentPart := versionPart(currentParts, index)
		if candidatePart > currentPart {
			return true
		}
		if candidatePart < currentPart {
			return false
		}
	}

	return false
}

func parseVersionParts(value string) ([]int, bool) {
	normalized := normalizeVersion(value)
	if normalized == "" {
		return nil, false
	}

	rawParts := strings.Split(normalized, ".")
	parts := make([]int, 0, len(rawParts))
	for _, rawPart := range rawParts {
		if rawPart == "" {
			return nil, false
		}

		part, err := strconv.Atoi(rawPart)
		if err != nil {
			return nil, false
		}
		parts = append(parts, part)
	}

	return parts, true
}

func versionPart(parts []int, index int) int {
	if index >= len(parts) {
		return 0
	}

	return parts[index]
}
