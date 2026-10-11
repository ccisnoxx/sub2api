package service

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrNoUpdateAvailable          = infraerrors.Conflict("ALREADY_UP_TO_DATE", "no update available; current version is latest")
	ErrRollbackVersionNotAllowed  = infraerrors.BadRequest("ROLLBACK_VERSION_NOT_ALLOWED", "version is not in the allowed rollback list")
	ErrInPlaceUpdateNotSupported  = infraerrors.Conflict("IN_PLACE_UPDATE_NOT_SUPPORTED", "in-place updates and rollbacks are not supported for this build; redeploy using its release source")
	personalReleaseVersionPattern = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)-klno\.(0|[1-9][0-9]*)-tps\.([1-9][0-9]*)$`)
)

const (
	updateCacheKey = "update_check_cache"
	updateCacheTTL = 1200 // 20 minutes
	// klno: 一键升级与回滚只认二开的发布；上游只做「有没有新版」的监测，绝不从上游下载——
	// 否则上游一发新版，点更新就会把二开换成上游二进制。
	githubRepo   = "KlN-4096/sub2api"
	upstreamRepo = "Wei-Shaw/sub2api"
	personalRepo = "ccisnoxx/sub2api"

	updateModeInPlace   = "in_place"
	updateModeManual    = "manual"
	updateModeContainer = "container"

	// Security: allowed download domains for updates
	allowedDownloadHost = "github.com"
	allowedAssetHost    = "objects.githubusercontent.com"

	// Security: max download size (500MB)
	maxDownloadSize = 500 * 1024 * 1024

	// Rollback: expose at most the 3 most recent versions older than current
	maxRollbackVersions = 3
	// Fetch a few extra releases so filtering (current/newer/prerelease) still leaves enough candidates
	rollbackFetchPageSize = 15
	// 个人发布包含预发布标签，获取 GitHub 单页上限后按版本序号筛选。
	personalReleaseFetchPageSize = 100
)

// UpdateCache defines cache operations for update service
type UpdateCache interface {
	GetUpdateInfo(ctx context.Context) (string, error)
	SetUpdateInfo(ctx context.Context, data string, ttl time.Duration) error
}

// GitHubReleaseClient 获取 GitHub release 信息的接口
type GitHubReleaseClient interface {
	FetchLatestRelease(ctx context.Context, repo string) (*GitHubRelease, error)
	FetchRecentReleases(ctx context.Context, repo string, perPage int) ([]*GitHubRelease, error)
	DownloadFile(ctx context.Context, url, dest string, maxSize int64) error
	FetchChecksumFile(ctx context.Context, url string) ([]byte, error)
}

// UpdateService handles software updates
type UpdateService struct {
	cache             UpdateCache
	githubClient      GitHubReleaseClient
	currentVersion    string
	buildType         string // "source" for manual builds, "release" for CI builds
	releaseRepository string
	updateMode        string
}

// NewUpdateService creates a new UpdateService
func NewUpdateService(cache UpdateCache, githubClient GitHubReleaseClient, version, buildType string) *UpdateService {
	repository, mode := githubRepo, updateModeManual
	if buildType == "release" {
		mode = updateModeInPlace
	}
	// 个人 tps 发布仅提供镜像；BuildType=release 不能授权替换容器内二进制。
	if strings.Contains(version, "-tps.") {
		repository = personalRepo
		if buildType == "release" {
			mode = updateModeContainer
		}
	}
	return &UpdateService{
		cache:             cache,
		githubClient:      githubClient,
		currentVersion:    version,
		buildType:         buildType,
		releaseRepository: repository,
		updateMode:        mode,
	}
}

// UpdateInfo contains update information
type UpdateInfo struct {
	CurrentVersion    string       `json:"current_version"`
	LatestVersion     string       `json:"latest_version"`
	HasUpdate         bool         `json:"has_update"`
	ReleaseInfo       *ReleaseInfo `json:"release_info,omitempty"`
	Cached            bool         `json:"cached"`
	Warning           string       `json:"warning,omitempty"`
	BuildType         string       `json:"build_type"`  // "source" or "release"
	UpdateMode        string       `json:"update_mode"` // "in_place", "manual" or "container"
	ReleaseRepository string       `json:"release_repository"`
	// Upstream 是上游仓库的只读监测结果；上面的字段都是二开自己的。
	Upstream *UpstreamUpdateInfo `json:"upstream,omitempty"`
}

// UpstreamUpdateInfo 上游版本监测：只提示，不提供升级。
type UpstreamUpdateInfo struct {
	CurrentVersion string `json:"current_version"` // 二开所基于的上游版本（去掉 -klno.N）
	LatestVersion  string `json:"latest_version"`
	HasUpdate      bool   `json:"has_update"`
	HTMLURL        string `json:"html_url,omitempty"`
	PublishedAt    string `json:"published_at,omitempty"`
	Warning        string `json:"warning,omitempty"`
}

// ReleaseInfo contains GitHub release details
type ReleaseInfo struct {
	Name        string  `json:"name"`
	Body        string  `json:"body"`
	PublishedAt string  `json:"published_at"`
	HTMLURL     string  `json:"html_url"`
	Assets      []Asset `json:"assets,omitempty"`
}

// Asset represents a release asset
type Asset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"download_url"`
	Size        int64  `json:"size"`
}

// GitHubRelease represents GitHub API response
type GitHubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Body        string        `json:"body"`
	PublishedAt string        `json:"published_at"`
	HTMLURL     string        `json:"html_url"`
	Draft       bool          `json:"draft"`
	Prerelease  bool          `json:"prerelease"`
	Assets      []GitHubAsset `json:"assets"`
}

// RollbackVersion describes a release version the system can roll back to
type RollbackVersion struct {
	Version     string `json:"version"` // without "v" prefix, e.g. "0.1.146"
	PublishedAt string `json:"published_at"`
	HTMLURL     string `json:"html_url"`
}

type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// CheckUpdate checks for available updates
func (s *UpdateService) CheckUpdate(ctx context.Context, force bool) (*UpdateInfo, error) {
	// Try cache first
	if !force {
		if cached, err := s.getFromCache(ctx); err == nil && cached != nil {
			return cached, nil
		}
	}

	// Fetch from GitHub
	info, err := s.fetchLatestRelease(ctx)
	if err != nil {
		// Return cached on error
		if cached, cacheErr := s.getFromCache(ctx); cacheErr == nil && cached != nil {
			cached.Warning = "Using cached data: " + err.Error()
			return cached, nil
		}
		info := s.newUpdateInfo(s.currentVersion, nil, false)
		info.Warning = err.Error()
		return info, nil
	}
	info.Upstream = s.fetchUpstreamInfo(ctx)

	// Cache result
	s.saveToCache(ctx, info)
	return info, nil
}

// fetchUpstreamInfo 查上游最新版本。失败只记 warning，不影响二开自己的检查结果。
func (s *UpdateService) fetchUpstreamInfo(ctx context.Context) *UpstreamUpdateInfo {
	current := upstreamBaseVersion(s.currentVersion)
	release, err := s.githubClient.FetchLatestRelease(ctx, upstreamRepo)
	if err != nil {
		return &UpstreamUpdateInfo{CurrentVersion: current, LatestVersion: current, Warning: err.Error()}
	}
	return s.upstreamInfo(upstreamBaseVersion(release.TagName), release.HTMLURL, release.PublishedAt)
}

func (s *UpdateService) upstreamInfo(latest, htmlURL, publishedAt string) *UpstreamUpdateInfo {
	current := upstreamBaseVersion(s.currentVersion)
	return &UpstreamUpdateInfo{
		CurrentVersion: current,
		LatestVersion:  latest,
		HasUpdate:      compareVersions(current, latest) < 0,
		HTMLURL:        htmlURL,
		PublishedAt:    publishedAt,
	}
}

// PerformUpdate downloads and applies the update
// Uses atomic file replacement pattern for safe in-place updates
func (s *UpdateService) PerformUpdate(ctx context.Context) error {
	if s.updateMode != updateModeInPlace {
		return ErrInPlaceUpdateNotSupported
	}
	info, err := s.CheckUpdate(ctx, true)
	if err != nil {
		return err
	}

	if !info.HasUpdate {
		return ErrNoUpdateAvailable
	}

	return s.applyReleaseAssets(ctx, info.ReleaseInfo.Assets)
}

// applyReleaseAssets downloads the platform archive from the given release assets,
// verifies its checksum, and atomically swaps the running binary.
// Shared by PerformUpdate (latest) and RollbackToVersion (specific older version).
func (s *UpdateService) applyReleaseAssets(ctx context.Context, releaseAssets []Asset) error {
	// Find matching archive and checksum for current platform
	archiveName := s.getArchiveName()
	var downloadURL string
	var checksumURL string

	for _, asset := range releaseAssets {
		if strings.Contains(asset.Name, archiveName) && !strings.HasSuffix(asset.Name, ".txt") {
			downloadURL = asset.DownloadURL
		}
		if asset.Name == "checksums.txt" {
			checksumURL = asset.DownloadURL
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("no compatible release found for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	// SECURITY: Validate download URL is from trusted domain
	if err := validateDownloadURL(downloadURL); err != nil {
		return fmt.Errorf("invalid download URL: %w", err)
	}
	if checksumURL != "" {
		if err := validateDownloadURL(checksumURL); err != nil {
			return fmt.Errorf("invalid checksum URL: %w", err)
		}
	}

	// Get current executable path
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("failed to resolve symlinks: %w", err)
	}

	exeDir := filepath.Dir(exePath)

	// Create temp directory in the SAME directory as executable
	// This ensures os.Rename is atomic (same filesystem)
	tempDir, err := os.MkdirTemp(exeDir, ".sub2api-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Download archive
	archivePath := filepath.Join(tempDir, filepath.Base(downloadURL))
	if err := s.downloadFile(ctx, downloadURL, archivePath); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	// Verify checksum if available
	if checksumURL != "" {
		if err := s.verifyChecksum(ctx, archivePath, checksumURL); err != nil {
			return fmt.Errorf("checksum verification failed: %w", err)
		}
	}

	// Extract binary from archive
	newBinaryPath := filepath.Join(tempDir, "sub2api")
	if err := s.extractBinary(archivePath, newBinaryPath); err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}

	// Set executable permission before replacement
	if err := os.Chmod(newBinaryPath, 0755); err != nil {
		return fmt.Errorf("chmod failed: %w", err)
	}

	// Atomic replacement using rename pattern:
	// 1. Rename current -> backup (atomic on Unix)
	// 2. Rename new -> current (atomic on Unix, same filesystem)
	// If step 2 fails, restore backup
	backupPath := exePath + ".backup"

	// Remove old backup if exists
	_ = os.Remove(backupPath)

	// Step 1: Move current binary to backup
	if err := os.Rename(exePath, backupPath); err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}

	// Step 2: Move new binary to target location (atomic, same filesystem)
	if err := os.Rename(newBinaryPath, exePath); err != nil {
		// Restore backup on failure
		if restoreErr := os.Rename(backupPath, exePath); restoreErr != nil {
			return fmt.Errorf("replace failed and restore failed: %w (restore error: %v)", err, restoreErr)
		}
		return fmt.Errorf("replace failed (restored backup): %w", err)
	}

	// Success - backup file is kept for rollback capability
	// It will be cleaned up on next successful update
	return nil
}

// Rollback restores the previous version
func (s *UpdateService) Rollback() error {
	if s.updateMode != updateModeInPlace {
		return ErrInPlaceUpdateNotSupported
	}
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("failed to resolve symlinks: %w", err)
	}

	backupFile := exePath + ".backup"
	if _, err := os.Stat(backupFile); os.IsNotExist(err) {
		return fmt.Errorf("no backup found")
	}

	// Replace current with backup
	if err := os.Rename(backupFile, exePath); err != nil {
		return fmt.Errorf("rollback failed: %w", err)
	}

	return nil
}

// ListRollbackVersions returns up to maxRollbackVersions release versions that are
// strictly older than the current version (the current version itself is excluded),
// newest first. Draft and prerelease entries are skipped.
func (s *UpdateService) ListRollbackVersions(ctx context.Context) ([]RollbackVersion, error) {
	if s.updateMode != updateModeInPlace {
		return nil, ErrInPlaceUpdateNotSupported
	}
	releases, err := s.fetchRollbackCandidates(ctx)
	if err != nil {
		return nil, err
	}

	versions := make([]RollbackVersion, 0, len(releases))
	for _, r := range releases {
		versions = append(versions, RollbackVersion{
			Version:     strings.TrimPrefix(r.TagName, "v"),
			PublishedAt: r.PublishedAt,
			HTMLURL:     r.HTMLURL,
		})
	}
	return versions, nil
}

// RollbackToVersion downloads and installs a specific older version.
// The target must be one of the versions returned by ListRollbackVersions;
// anything else (including the current version) is rejected.
func (s *UpdateService) RollbackToVersion(ctx context.Context, version string) error {
	if s.updateMode != updateModeInPlace {
		return ErrInPlaceUpdateNotSupported
	}
	target := strings.TrimPrefix(strings.TrimSpace(version), "v")
	if target == "" {
		return ErrRollbackVersionNotAllowed
	}

	releases, err := s.fetchRollbackCandidates(ctx)
	if err != nil {
		return err
	}

	var match *GitHubRelease
	for _, r := range releases {
		if strings.TrimPrefix(r.TagName, "v") == target {
			match = r
			break
		}
	}
	if match == nil {
		return ErrRollbackVersionNotAllowed
	}

	assets := make([]Asset, len(match.Assets))
	for i, a := range match.Assets {
		assets[i] = Asset{
			Name:        a.Name,
			DownloadURL: a.BrowserDownloadURL,
			Size:        a.Size,
		}
	}

	return s.applyReleaseAssets(ctx, assets)
}

// fetchRollbackCandidates fetches recent releases and keeps the newest
// maxRollbackVersions entries strictly older than the current version.
func (s *UpdateService) fetchRollbackCandidates(ctx context.Context) ([]*GitHubRelease, error) {
	releases, err := s.githubClient.FetchRecentReleases(ctx, s.releaseRepository, rollbackFetchPageSize)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool, len(releases))
	candidates := make([]*GitHubRelease, 0, maxRollbackVersions)
	for _, r := range releases {
		if r == nil || r.Draft || r.Prerelease {
			continue
		}
		v := strings.TrimPrefix(r.TagName, "v")
		if v == "" || seen[v] {
			continue
		}
		// Only versions strictly older than current (also excludes current itself)
		if compareVersions(v, s.currentVersion) >= 0 {
			continue
		}
		seen[v] = true
		candidates = append(candidates, r)
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return compareVersions(
			strings.TrimPrefix(candidates[i].TagName, "v"),
			strings.TrimPrefix(candidates[j].TagName, "v"),
		) > 0
	})

	if len(candidates) > maxRollbackVersions {
		candidates = candidates[:maxRollbackVersions]
	}
	return candidates, nil
}

func (s *UpdateService) fetchLatestRelease(ctx context.Context) (*UpdateInfo, error) {
	var release *GitHubRelease
	if s.releaseRepository == personalRepo {
		// /releases/latest 排除预发布；个人 tps 发布需要从列表中挑选。
		releases, err := s.githubClient.FetchRecentReleases(ctx, s.releaseRepository, personalReleaseFetchPageSize)
		if err != nil {
			return nil, err
		}
		for _, candidate := range releases {
			if candidate == nil || candidate.Draft || !personalReleaseVersionPattern.MatchString(candidate.TagName) {
				continue
			}
			if release == nil || compareVersions(candidate.TagName, release.TagName) > 0 {
				release = candidate
			}
		}
		if release == nil {
			return nil, fmt.Errorf("未找到已发布的个人 tps 版本")
		}
	} else {
		var err error
		release, err = s.githubClient.FetchLatestRelease(ctx, s.releaseRepository)
		if err != nil {
			return nil, err
		}
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	if err := s.validateReleaseVersion(latestVersion); err != nil {
		return nil, err
	}

	assets := make([]Asset, len(release.Assets))
	for i, a := range release.Assets {
		assets[i] = Asset{
			Name:        a.Name,
			DownloadURL: a.BrowserDownloadURL,
			Size:        a.Size,
		}
	}

	return s.newUpdateInfo(latestVersion, &ReleaseInfo{
		Name:        release.Name,
		Body:        release.Body,
		PublishedAt: release.PublishedAt,
		HTMLURL:     release.HTMLURL,
		Assets:      assets,
	}, false), nil
}

func (s *UpdateService) newUpdateInfo(latest string, releaseInfo *ReleaseInfo, cached bool) *UpdateInfo {
	return &UpdateInfo{
		CurrentVersion:    s.currentVersion,
		LatestVersion:     latest,
		HasUpdate:         compareVersions(s.currentVersion, latest) < 0,
		ReleaseInfo:       releaseInfo,
		Cached:            cached,
		BuildType:         s.buildType,
		UpdateMode:        s.updateMode,
		ReleaseRepository: s.releaseRepository,
	}
}

func (s *UpdateService) validateReleaseVersion(version string) error {
	if s.releaseRepository == personalRepo && !personalReleaseVersionPattern.MatchString(version) {
		return fmt.Errorf("personal release tag does not match X.Y.Z-klno.N-tps.N: %s", version)
	}
	return nil
}

func (s *UpdateService) downloadFile(ctx context.Context, downloadURL, dest string) error {
	return s.githubClient.DownloadFile(ctx, downloadURL, dest, maxDownloadSize)
}

func (s *UpdateService) getArchiveName() string {
	osName := runtime.GOOS
	arch := runtime.GOARCH
	return fmt.Sprintf("%s_%s", osName, arch)
}

// validateDownloadURL checks if the URL is from an allowed domain
// SECURITY: This prevents SSRF and ensures downloads only come from trusted GitHub domains
func validateDownloadURL(rawURL string) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Must be HTTPS
	if parsedURL.Scheme != "https" {
		return fmt.Errorf("only HTTPS URLs are allowed")
	}

	// Check against allowed hosts
	host := parsedURL.Host
	// GitHub release URLs can be from github.com or objects.githubusercontent.com
	if host != allowedDownloadHost &&
		!strings.HasSuffix(host, "."+allowedDownloadHost) &&
		host != allowedAssetHost &&
		!strings.HasSuffix(host, "."+allowedAssetHost) {
		return fmt.Errorf("download from untrusted host: %s", host)
	}

	return nil
}

func (s *UpdateService) verifyChecksum(ctx context.Context, filePath, checksumURL string) error {
	// Download checksums file
	checksumData, err := s.githubClient.FetchChecksumFile(ctx, checksumURL)
	if err != nil {
		return fmt.Errorf("failed to download checksums: %w", err)
	}

	// Calculate file hash
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	actualHash := hex.EncodeToString(h.Sum(nil))

	// Find expected hash in checksums file
	fileName := filepath.Base(filePath)
	scanner := bufio.NewScanner(strings.NewReader(string(checksumData)))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == fileName {
			if parts[0] == actualHash {
				return nil
			}
			return fmt.Errorf("checksum mismatch: expected %s, got %s", parts[0], actualHash)
		}
	}

	return fmt.Errorf("checksum not found for %s", fileName)
}

func (s *UpdateService) extractBinary(archivePath, destPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	var reader io.Reader = f

	// Handle gzip compression
	if strings.HasSuffix(archivePath, ".gz") || strings.HasSuffix(archivePath, ".tar.gz") || strings.HasSuffix(archivePath, ".tgz") {
		gzr, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer func() { _ = gzr.Close() }()
		reader = gzr
	}

	// Handle tar archive
	if strings.Contains(archivePath, ".tar") {
		tr := tar.NewReader(reader)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}

			// SECURITY: Prevent Zip Slip / Path Traversal attack
			// Only allow files with safe base names, no directory traversal
			baseName := filepath.Base(hdr.Name)

			// Check for path traversal attempts
			if strings.Contains(hdr.Name, "..") {
				return fmt.Errorf("path traversal attempt detected: %s", hdr.Name)
			}

			// Validate the entry is a regular file
			if hdr.Typeflag != tar.TypeReg {
				continue // Skip directories and special files
			}

			// Only extract the specific binary we need
			if baseName == "sub2api" || baseName == "sub2api.exe" {
				// Additional security: limit file size (max 500MB)
				const maxBinarySize = 500 * 1024 * 1024
				if hdr.Size > maxBinarySize {
					return fmt.Errorf("binary too large: %d bytes (max %d)", hdr.Size, maxBinarySize)
				}

				out, err := os.Create(destPath)
				if err != nil {
					return err
				}

				// Use LimitReader to prevent decompression bombs
				limited := io.LimitReader(tr, maxBinarySize)
				if _, err := io.Copy(out, limited); err != nil {
					_ = out.Close()
					return err
				}
				if err := out.Close(); err != nil {
					return err
				}
				return nil
			}
		}
		return fmt.Errorf("binary not found in archive")
	}

	// Direct copy for non-tar files (with size limit)
	const maxBinarySize = 500 * 1024 * 1024
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}

	limited := io.LimitReader(reader, maxBinarySize)
	if _, err := io.Copy(out, limited); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func (s *UpdateService) getFromCache(ctx context.Context) (*UpdateInfo, error) {
	data, err := s.cache.GetUpdateInfo(ctx)
	if err != nil {
		return nil, err
	}

	var cached updateCacheData
	if err := json.Unmarshal([]byte(data), &cached); err != nil {
		return nil, err
	}
	// 发布通道迁移后不能复用 KlN/原版缓存来判断个人镜像是否有更新。
	if cached.Repository != s.releaseRepository {
		return nil, fmt.Errorf("update cache belongs to a different release repository")
	}
	if err := s.validateReleaseVersion(cached.Latest); err != nil {
		return nil, err
	}

	if time.Now().Unix()-cached.Timestamp > updateCacheTTL {
		return nil, fmt.Errorf("cache expired")
	}

	info := s.newUpdateInfo(cached.Latest, cached.ReleaseInfo, true)
	// 当前版本可能在缓存期内变了（升级后重启），has_update 按当前版本重算。
	if u := cached.Upstream; u != nil {
		info.Upstream = s.upstreamInfo(u.LatestVersion, u.HTMLURL, u.PublishedAt)
		info.Upstream.Warning = u.Warning
	}
	return info, nil
}

type updateCacheData struct {
	Repository  string              `json:"repository"`
	Latest      string              `json:"latest"`
	ReleaseInfo *ReleaseInfo        `json:"release_info"`
	Upstream    *UpstreamUpdateInfo `json:"upstream,omitempty"`
	Timestamp   int64               `json:"timestamp"`
}

func (s *UpdateService) saveToCache(ctx context.Context, info *UpdateInfo) {
	cacheData := updateCacheData{
		Repository:  s.releaseRepository,
		Latest:      info.LatestVersion,
		ReleaseInfo: info.ReleaseInfo,
		Upstream:    info.Upstream,
		Timestamp:   time.Now().Unix(),
	}

	data, _ := json.Marshal(cacheData)
	_ = s.cache.SetUpdateInfo(ctx, string(data), time.Duration(updateCacheTTL)*time.Second)
}

// compareVersions 比较 X.Y.Z[-klno.N[-tps.N]]：依次比较基础版本、二开序号与个人序号。
// 其它后缀（如 -rc1）照旧忽略。
func compareVersions(current, latest string) int {
	currentParts := parseVersion(current)
	latestParts := parseVersion(latest)

	for i := range currentParts {
		if currentParts[i] < latestParts[i] {
			return -1
		}
		if currentParts[i] > latestParts[i] {
			return 1
		}
	}
	return 0
}

func parseVersion(v string) [5]int {
	base, suffix, _ := strings.Cut(strings.TrimPrefix(v, "v"), "-")
	var result [5]int
	parts := strings.Split(base, ".")
	for i := 0; i < len(parts) && i < 3; i++ {
		if parsed, err := strconv.Atoi(parts[i]); err == nil {
			result[i] = parsed
		}
	}
	if n, ok := strings.CutPrefix(suffix, "klno."); ok {
		klno, tps, _ := strings.Cut(n, "-tps.")
		if parsed, err := strconv.Atoi(klno); err == nil {
			result[3] = parsed
		}
		if parsed, err := strconv.Atoi(tps); err == nil {
			result[4] = parsed
		}
	}
	return result
}

// upstreamBaseVersion 去掉二开后缀，得到所基于的上游版本号。
func upstreamBaseVersion(v string) string {
	base, _, _ := strings.Cut(strings.TrimPrefix(strings.TrimSpace(v), "v"), "-")
	return base
}
