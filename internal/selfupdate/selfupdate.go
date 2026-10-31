// Package selfupdate provides in-place updates for managed Skillstation installations.
package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/vantt/skillstation/internal/version"
)

const (
	// DefaultReleaseBaseURL is the standard GitHub releases base URL.
	DefaultReleaseBaseURL = "https://github.com/vantt/skillstation/releases"

	// DefaultSigstoreIssuer is the GitHub Actions OIDC token issuer.
	DefaultSigstoreIssuer = "https://token.actions.githubusercontent.com"

	// DefaultSigstoreWorkflow is the release workflow identity prefix.
	DefaultSigstoreWorkflow = "https://github.com/vantt/skillstation/.github/workflows/release.yml"

	// ManagedMarkerHeader is the expected first line of .skillstation-managed.
	ManagedMarkerHeader = "skillstation-managed-v1"

	// Bounded limits for downloads
	maxChecksumSize = 1024 * 1024 // 1 MB
	maxBundleSize   = 1024 * 1024 // 1 MB
	maxArchiveSize  = 134217728   // 128 MB
)

// Result describes the outcome of a self-update operation.
type Result struct {
	CurrentVersion  string `json:"current_version"`
	TargetVersion   string `json:"target_version"`
	Updated         bool   `json:"updated"`
	AlreadyUpToDate bool   `json:"already_up_to_date"`
	CheckOnly       bool   `json:"check_only"`
	Message         string `json:"message"`
}

// Error represents an actionable failure during selfupdate.
type Error struct {
	Summary string
	Why     string
	Fix     string
	Err     error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Summary, e.Err)
	}
	return e.Summary
}

// Options configure the self-update execution.
type Options struct {
	// CurrentVersion overrides the running version (defaults to version.Current().Version).
	CurrentVersion string

	// TargetVersion specifies an explicit target version (e.g. "0.2.0" or "v0.2.0").
	// If empty, the latest stable release is fetched.
	TargetVersion string

	// ExecutablePath overrides the path of the current binary (defaults to os.Executable()).
	ExecutablePath string

	// BaseURL overrides the releases base URL (env SKILLSTATION_UPDATE_BASE_URL).
	BaseURL string

	// CheckOnly when true only checks for updates without downloading or replacing the binary.
	CheckOnly bool

	// RequireSig enforces cosign signature verification (also set by SKILLSTATION_REQUIRE_SIGNATURE=1).
	RequireSig bool

	// HTTPClient overrides the HTTP client for network calls.
	HTTPClient *http.Client

	// Stdout receives non-error progress output.
	Stdout io.Writer

	// Stderr receives warnings.
	Stderr io.Writer

	// GOOS overrides runtime.GOOS (for tests).
	GOOS string

	// GOARCH overrides runtime.GOARCH (for tests).
	GOARCH string

	// CosignVerifier overrides signature verification execution (for tests).
	CosignVerifier func(ctx context.Context, manifestPath, bundlePath, identity, issuer string) error

	// BinaryVerifier overrides post-replacement binary verification (for tests).
	BinaryVerifier func(ctx context.Context, exePath string) error
}

// Run executes the self-update or check process.
func Run(ctx context.Context, opts Options) (*Result, error) {
	// 1. Resolve executable and check managed marker
	exePath, err := resolveExecutablePath(opts.ExecutablePath)
	if err != nil {
		return nil, &Error{
			Summary: "Could not determine executable path.",
			Why:     err.Error(),
			Fix:     "Ensure skillstation is executed from a valid filesystem path.",
		}
	}

	binDir := filepath.Dir(exePath)
	markerPath := filepath.Join(binDir, ".skillstation-managed")
	markerVersion, err := verifyManagedMarker(markerPath, exePath)
	if err != nil {
		return nil, err
	}

	// Clean up any stale Windows .old file from previous update run
	cleanOldBinary(exePath)

	// 2. Resolve current version
	normalizedCurrent := resolveCurrentVersion(opts.CurrentVersion, markerVersion)

	// 3. Resolve target version & release tag
	baseURL := resolveBaseURL(opts.BaseURL)
	targetVersion, releaseTag, err := resolveTarget(ctx, opts, baseURL)
	if err != nil {
		return nil, err
	}
	normalizedTarget := strings.TrimPrefix(strings.TrimSpace(targetVersion), "v")

	// 4. Compare versions
	if result := checkResult(opts, normalizedCurrent, normalizedTarget); result != nil {
		return result, nil
	}

	// 5. Check signature verification requirements
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	release := releaseDownload{client: client, baseURL: baseURL, tag: releaseTag}

	requireSig := opts.RequireSig || os.Getenv("SKILLSTATION_REQUIRE_SIGNATURE") == "1"
	cosignPath := findCosignPath()
	if opts.CosignVerifier == nil {
		opts.CosignVerifier = fixtureVerifier(baseURL)
	}
	hasCosign := cosignPath != "" || opts.CosignVerifier != nil

	if requireSig && !hasCosign {
		return nil, &Error{
			Summary: "Signature verification is required but cosign is not installed.",
			Why:     "SKILLSTATION_REQUIRE_SIGNATURE is set, but cosign was not found on PATH.",
			Fix:     "Install cosign from https://github.com/sigstore/cosign to continue, or unset SKILLSTATION_REQUIRE_SIGNATURE.",
		}
	}

	// 6. Determine archive asset name
	asset := platformAsset(opts, normalizedTarget)

	// 7. Download checksums.txt
	checksumURL := release.assetURL("checksums.txt")
	checksumBytes, err := downloadBytes(ctx, client, checksumURL, maxChecksumSize)
	if err != nil {
		return nil, &Error{
			Summary: "Failed to download checksums manifest.",
			Why:     fmt.Sprintf("Could not retrieve %s: %v", checksumURL, err),
			Fix:     "Check your network connection and retry the update.",
			Err:     err,
		}
	}

	// 8. Signature verification if cosign available
	if err := authenticateChecksums(ctx, opts, release, checksumBytes, cosignPath, hasCosign); err != nil {
		return nil, err
	}

	// 9. Verify archive SHA-256 against checksums.txt
	archiveBytes, err := downloadVerifiedArchive(ctx, opts, release, checksumBytes, asset.archiveName)
	if err != nil {
		return nil, err
	}

	// 10. Extract binary only (validate member name to prevent path traversal)
	newBinaryBytes, err := extractBinary(archiveBytes, asset.ext, asset.binaryName)
	if err != nil {
		return nil, &Error{
			Summary: "Failed to extract binary from archive.",
			Why:     err.Error(),
			Fix:     "The release archive format or contents are invalid. Report this issue.",
			Err:     err,
		}
	}

	// 11. Atomic replacement with rollback
	if err := replaceBinaryWithRollback(ctx, opts, exePath, markerPath, newBinaryBytes, normalizedTarget); err != nil {
		return nil, err
	}

	return &Result{
		CurrentVersion:  normalizedCurrent,
		TargetVersion:   normalizedTarget,
		Updated:         true,
		AlreadyUpToDate: false,
		CheckOnly:       false,
		Message:         fmt.Sprintf("Updated %s -> %s.", normalizedCurrent, normalizedTarget),
	}, nil
}

// resolveCurrentVersion returns the running version without a leading v: the
// override, else the build version, else the marker's version for a dev
// build, else dev.
func resolveCurrentVersion(override, markerVersion string) string {
	currentVersion := override
	if currentVersion == "" {
		currentVersion = version.Current().Version
	}
	if (currentVersion == "" || currentVersion == "dev") && markerVersion != "" {
		currentVersion = markerVersion
	}
	if currentVersion == "" {
		currentVersion = "dev"
	}
	return strings.TrimPrefix(strings.TrimSpace(currentVersion), "v")
}

// checkResult returns the result when there is nothing to install: the
// installation is up to date, or only a check was asked for. It returns nil
// when the update should proceed.
func checkResult(opts Options, normalizedCurrent, normalizedTarget string) *Result {
	cmp, hasSemver := CompareVersions(normalizedCurrent, normalizedTarget)

	var upToDate bool
	if strings.TrimSpace(opts.TargetVersion) != "" {
		// When explicit target is given, target == current means already up to date
		upToDate = (hasSemver && cmp == 0) || normalizedCurrent == normalizedTarget
	} else {
		// When resolving latest, if current >= target, already up to date
		upToDate = (hasSemver && cmp >= 0) || (!hasSemver && normalizedCurrent == normalizedTarget)
	}
	if upToDate {
		return &Result{
			CurrentVersion:  normalizedCurrent,
			TargetVersion:   normalizedTarget,
			Updated:         false,
			AlreadyUpToDate: true,
			CheckOnly:       opts.CheckOnly,
			Message:         fmt.Sprintf("Already up to date (%s).", normalizedCurrent),
		}
	}

	// An update is available
	if opts.CheckOnly {
		return &Result{
			CurrentVersion:  normalizedCurrent,
			TargetVersion:   normalizedTarget,
			Updated:         false,
			AlreadyUpToDate: false,
			CheckOnly:       true,
			Message:         fmt.Sprintf("Update available: %s -> %s. Run `skillstation update` to install.", normalizedCurrent, normalizedTarget),
		}
	}
	return nil
}

// releaseDownload names a release on the download server and the client that
// fetches its assets.
type releaseDownload struct {
	client  *http.Client
	baseURL string
	tag     string
}

func (release releaseDownload) assetURL(name string) string {
	return fmt.Sprintf("%s/download/%s/%s", release.baseURL, release.tag, name)
}

// releaseAsset is the archive of one platform and the binary inside it.
type releaseAsset struct {
	archiveName, ext, binaryName string
}

func platformAsset(opts Options, normalizedTarget string) releaseAsset {
	goos := opts.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	goarch := opts.GOARCH
	if goarch == "" {
		goarch = runtime.GOARCH
	}

	ext := "tar.gz"
	binaryName := "skillstation"
	if goos == "windows" {
		ext = "zip"
		binaryName = "skillstation.exe"
	}
	return releaseAsset{
		archiveName: fmt.Sprintf("skillstation-%s-%s-%s.%s", normalizedTarget, goos, goarch, ext),
		ext:         ext,
		binaryName:  binaryName,
	}
}

// sigstoreSigner is the certificate identity and OIDC issuer a release
// signature must carry.
type sigstoreSigner struct {
	identity, issuer string
}

// authenticateChecksums verifies the Sigstore bundle of checksums.txt when a
// verifier is available, and otherwise prints how to verify it by hand.
func authenticateChecksums(ctx context.Context, opts Options, release releaseDownload, checksumBytes []byte, cosignPath string, hasCosign bool) error {
	sigstoreWorkflow := os.Getenv("SKILLSTATION_SIGSTORE_WORKFLOW")
	if sigstoreWorkflow == "" {
		sigstoreWorkflow = DefaultSigstoreWorkflow
	}
	sigstoreIssuer := os.Getenv("SKILLSTATION_SIGSTORE_ISSUER")
	if sigstoreIssuer == "" {
		sigstoreIssuer = DefaultSigstoreIssuer
	}
	signer := sigstoreSigner{identity: fmt.Sprintf("%s@refs/tags/%s", sigstoreWorkflow, release.tag), issuer: sigstoreIssuer}

	if !hasCosign {
		if opts.Stdout != nil {
			fmt.Fprintf(opts.Stdout, "Signature not checked (cosign not installed). To verify: cosign verify-blob --bundle checksums.txt.sigstore.json --certificate-identity %s --certificate-oidc-issuer %s checksums.txt\n", signer.identity, signer.issuer)
		}
		return nil
	}
	bundleURL := release.assetURL("checksums.txt.sigstore.json")
	bundleBytes, bErr := downloadBytes(ctx, release.client, bundleURL, maxBundleSize)
	if bErr != nil {
		return &Error{
			Summary: "Failed to download signature bundle.",
			Why:     fmt.Sprintf("Could not retrieve %s: %v", bundleURL, bErr),
			Fix:     "Ensure the release assets include checksums.txt.sigstore.json and retry.",
			Err:     bErr,
		}
	}

	if err := verifySignature(ctx, opts.CosignVerifier, cosignPath, checksumBytes, bundleBytes, signer); err != nil {
		return &Error{
			Summary: "Signature verification failed.",
			Why:     fmt.Sprintf("Sigstore verification failed for checksums.txt: %v", err),
			Fix:     "Verify that cosign is functioning properly and release signatures are valid.",
			Err:     err,
		}
	}
	if opts.Stdout != nil {
		fmt.Fprintln(opts.Stdout, "Authenticated checksums.txt with Sigstore bundle")
	}
	return nil
}

// downloadVerifiedArchive downloads the release archive and checks its SHA-256
// against the entry in checksums.txt.
func downloadVerifiedArchive(ctx context.Context, opts Options, release releaseDownload, checksumBytes []byte, archiveName string) ([]byte, error) {
	expectedSHA, err := parseChecksums(checksumBytes, archiveName)
	if err != nil {
		return nil, &Error{
			Summary: "Checksum entry not found.",
			Why:     fmt.Sprintf("checksums.txt does not contain an entry for %s: %v", archiveName, err),
			Fix:     "Ensure the release includes an archive for this platform and retry.",
			Err:     err,
		}
	}

	archiveURL := release.assetURL(archiveName)
	archiveBytes, err := downloadBytes(ctx, release.client, archiveURL, maxArchiveSize)
	if err != nil {
		return nil, &Error{
			Summary: "Failed to download release archive.",
			Why:     fmt.Sprintf("Could not retrieve %s: %v", archiveURL, err),
			Fix:     "Check your network connection and retry the update.",
			Err:     err,
		}
	}

	actualSHA := fmt.Sprintf("%x", sha256.Sum256(archiveBytes))
	if !strings.EqualFold(actualSHA, expectedSHA) {
		return nil, &Error{
			Summary: "Checksum verification failed.",
			Why:     fmt.Sprintf("SHA-256 verification failed for %s: expected %s, got %s", archiveName, expectedSHA, actualSHA),
			Fix:     "The downloaded artifact does not match published checksums. Check your connection or cache and retry.",
		}
	}
	if opts.Stdout != nil {
		fmt.Fprintf(opts.Stdout, "Verified SHA-256 for %s\n", archiveName)
	}
	return archiveBytes, nil
}

func resolveExecutablePath(override string) (string, error) {
	exePath := override
	if exePath == "" {
		exePath = os.Getenv("SKILLSTATION_UPDATE_TEST_EXECUTABLE")
	}
	if exePath == "" {
		var err error
		exePath, err = os.Executable()
		if err != nil {
			return "", err
		}
	}
	if realPath, err := filepath.EvalSymlinks(exePath); err == nil {
		exePath = realPath
	}
	return filepath.Clean(exePath), nil
}

func verifyManagedMarker(markerPath, exePath string) (string, error) {
	fi, err := os.Lstat(markerPath)
	if err != nil {
		return "", &Error{
			Summary: "Refusing to update an unmanaged binary.",
			Why:     fmt.Sprintf("The binary at %s was not installed by the Skillstation installer (missing .skillstation-managed marker).", exePath),
			Fix: "To reinstall or manage Skillstation with the official installer:\n" +
				"  Linux/macOS: curl -fsSL https://github.com/vantt/skillstation/releases/latest/download/install.sh | sh\n" +
				"  Windows:     powershell -ExecutionPolicy ByPass -c \"irm https://github.com/vantt/skillstation/releases/latest/download/install.ps1 | iex\"",
		}
	}

	if fi.Mode()&os.ModeSymlink != 0 {
		return "", &Error{
			Summary: "Refusing to update an unmanaged binary.",
			Why:     fmt.Sprintf("The installation marker at %s is a symlink.", markerPath),
			Fix:     "Reinstall Skillstation with the official installer.",
		}
	}

	data, err := os.ReadFile(markerPath)
	if err != nil {
		return "", &Error{
			Summary: "Could not read installation marker.",
			Why:     err.Error(),
			Fix:     "Check file permissions on .skillstation-managed.",
			Err:     err,
		}
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != ManagedMarkerHeader {
		return "", &Error{
			Summary: "Refusing to update an unmanaged binary.",
			Why:     fmt.Sprintf("The marker at %s does not contain a valid %q header.", markerPath, ManagedMarkerHeader),
			Fix: "To reinstall or manage Skillstation with the official installer:\n" +
				"  Linux/macOS: curl -fsSL https://github.com/vantt/skillstation/releases/latest/download/install.sh | sh\n" +
				"  Windows:     powershell -ExecutionPolicy ByPass -c \"irm https://github.com/vantt/skillstation/releases/latest/download/install.ps1 | iex\"",
		}
	}

	markerVersion := ""
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "version=") {
			markerVersion = strings.TrimPrefix(line, "version=")
			break
		}
	}

	return markerVersion, nil
}

func cleanOldBinary(exePath string) {
	oldExe := exePath + ".old"
	if _, err := os.Stat(oldExe); err == nil {
		_ = os.Remove(oldExe)
	}
}

func resolveBaseURL(override string) string {
	baseURL := override
	if baseURL == "" {
		baseURL = os.Getenv("SKILLSTATION_UPDATE_BASE_URL")
	}
	if baseURL == "" {
		baseURL = DefaultReleaseBaseURL
	}
	return strings.TrimRight(baseURL, "/")
}

// maxLatestRedirects bounds the redirects followed while looking for the latest release tag.
const maxLatestRedirects = 5

func resolveTarget(ctx context.Context, opts Options, baseURL string) (string, string, error) {
	if strings.TrimSpace(opts.TargetVersion) != "" {
		raw := strings.TrimSpace(opts.TargetVersion)
		ver := strings.TrimPrefix(raw, "v")
		return ver, "v" + ver, nil
	}

	latestURL := baseURL + "/latest"
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}

	// A renamed repository answers with a redirect to the new repository's /releases/latest before the
	// redirect that names the tag, so follow redirects until one carries a tag.
	var rawTag string
	current := latestURL
	for hop := 0; ; hop++ {
		if hop > maxLatestRedirects {
			return "", "", &Error{
				Summary: "Failed to resolve latest release.",
				Why:     fmt.Sprintf("More than %d redirects while checking %s", maxLatestRedirects, latestURL),
				Fix:     "Specify an explicit target version via `skillstation update --version <v>`.",
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, current, nil)
		if err != nil {
			return "", "", &Error{
				Summary: "Could not create request for latest release.",
				Why:     err.Error(),
				Fix:     "Check the update configuration and retry.",
				Err:     err,
			}
		}

		resp, err := client.Do(req)
		if err != nil {
			return "", "", &Error{
				Summary: "Failed to resolve latest release.",
				Why:     fmt.Sprintf("Request to %s failed: %v", current, err),
				Fix:     "Check your internet connection and retry.",
				Err:     err,
			}
		}
		_ = resp.Body.Close()

		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			loc := resp.Header.Get("Location")
			if loc == "" {
				return "", "", &Error{
					Summary: "Failed to resolve latest release.",
					Why:     "Server returned redirect with missing Location header.",
					Fix:     "Retry the update or specify --version <v>.",
				}
			}
			next, err := req.URL.Parse(loc)
			if err != nil {
				return "", "", &Error{
					Summary: "Failed to parse release tag from redirect.",
					Why:     err.Error(),
					Fix:     "Specify an explicit target version via `skillstation update --version <v>`.",
					Err:     err,
				}
			}
			if rawTag, err = extractTagFromURL(next.String()); err == nil {
				break
			}
			if !strings.HasSuffix(strings.TrimRight(next.Path, "/"), "/latest") {
				return "", "", &Error{
					Summary: "Failed to parse release tag from redirect.",
					Why:     err.Error(),
					Fix:     "Specify an explicit target version via `skillstation update --version <v>`.",
					Err:     err,
				}
			}
			current = next.String()
			continue
		}
		if resp.StatusCode == http.StatusOK {
			if resp.Request != nil && resp.Request.URL != nil {
				var err error
				rawTag, err = extractTagFromURL(resp.Request.URL.String())
				if err != nil {
					return "", "", &Error{
						Summary: "Failed to parse release tag from URL.",
						Why:     err.Error(),
						Fix:     "Specify an explicit target version via `skillstation update --version <v>`.",
						Err:     err,
					}
				}
				break
			}
			return "", "", &Error{
				Summary: "Failed to resolve latest release.",
				Why:     fmt.Sprintf("Unexpected status 200 without redirect when checking %s", current),
				Fix:     "Specify an explicit target version via `skillstation update --version <v>`.",
			}
		}
		return "", "", &Error{
			Summary: "Failed to resolve latest release.",
			Why:     fmt.Sprintf("HTTP status %d when checking %s", resp.StatusCode, current),
			Fix:     "Check your network connection or specify --version <v>.",
		}
	}

	targetVer := strings.TrimPrefix(rawTag, "v")
	releaseTag := rawTag
	if !strings.HasPrefix(releaseTag, "v") {
		releaseTag = "v" + releaseTag
	}
	return targetVer, releaseTag, nil
}

func extractTagFromURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i := range len(parts) - 1 {
		if parts[i] == "tag" {
			return parts[i+1], nil
		}
	}
	if last := parts[len(parts)-1]; last != "" && last != "latest" {
		return last, nil
	}
	return "", fmt.Errorf("could not determine release tag from URL path: %s", u.Path)
}

func findCosignPath() string {
	cosignPath, err := exec.LookPath("cosign")
	if err == nil {
		return cosignPath
	}
	if runtime.GOOS == "windows" {
		cosignPath, err = exec.LookPath("cosign.exe")
		if err == nil {
			return cosignPath
		}
	}
	return ""
}

func verifySignature(ctx context.Context, verifier func(ctx context.Context, m, b, id, iss string) error, cosignPath string, manifestBytes, bundleBytes []byte, signer sigstoreSigner) error {
	identity, issuer := signer.identity, signer.issuer
	tmpDir, err := os.MkdirTemp("", "skillstation-update-sig-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	manifestPath := filepath.Join(tmpDir, "checksums.txt")
	bundlePath := filepath.Join(tmpDir, "checksums.txt.sigstore.json")

	if err := os.WriteFile(manifestPath, manifestBytes, 0600); err != nil {
		return err
	}
	if err := os.WriteFile(bundlePath, bundleBytes, 0600); err != nil {
		return err
	}

	if verifier != nil {
		return verifier(ctx, manifestPath, bundlePath, identity, issuer)
	}

	if cosignPath == "" {
		return errors.New("cosign binary not found")
	}

	cmd := exec.CommandContext(ctx, cosignPath, "verify-blob",
		"--bundle", bundlePath,
		"--certificate-identity", identity,
		"--certificate-oidc-issuer", issuer,
		manifestPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// fixtureVerifier returns the stand-in for cosign named by
// SKILLSTATION_FIXTURE_VERIFIER, which installer tests use. Like the installer,
// which honours it only in local fixture mode, it is used only for a release
// served from this machine; a release from anywhere else is always checked by
// cosign.
func fixtureVerifier(baseURL string) func(ctx context.Context, manifestPath, bundlePath, identity, issuer string) error {
	program := os.Getenv("SKILLSTATION_FIXTURE_VERIFIER")
	if program == "" || !localReleaseURL(baseURL) {
		return nil
	}
	return func(ctx context.Context, manifestPath, bundlePath, identity, issuer string) error {
		out, err := exec.CommandContext(ctx, program, manifestPath, bundlePath, identity, issuer).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}
}

// localReleaseURL reports whether raw names a file URL or an HTTP(S) server on
// a loopback address.
func localReleaseURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	switch parsed.Scheme {
	case "file":
		return parsed.Host == "" || parsed.Host == "localhost"
	case "http", "https":
	default:
		return false
	}
	host := parsed.Hostname()
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func downloadBytes(ctx context.Context, client *http.Client, urlStr string, maxBytes int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP status %d", resp.StatusCode)
	}

	lr := io.LimitReader(resp.Body, maxBytes+1)
	data, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("downloaded data exceeded maximum allowed size (%d bytes)", maxBytes)
	}
	return data, nil
}

func parseChecksums(data []byte, targetFile string) (string, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	var foundHash string
	count := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		hash := fields[0]
		fileName := strings.TrimPrefix(fields[1], "*")
		fileName = filepath.Base(fileName)

		if fileName == targetFile {
			if len(hash) != 64 {
				return "", fmt.Errorf("invalid SHA-256 hash length in checksum entry: %s", hash)
			}
			foundHash = hash
			count++
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("reading checksums.txt: %w", err)
	}
	if count == 0 {
		return "", fmt.Errorf("no entry found for %s", targetFile)
	}
	if count > 1 {
		return "", fmt.Errorf("multiple entries found for %s", targetFile)
	}
	return foundHash, nil
}

func extractBinary(archiveBytes []byte, ext, binaryName string) ([]byte, error) {
	switch ext {
	case "tar.gz":
		return extractTarGz(archiveBytes, binaryName)
	case "zip":
		return extractZip(archiveBytes, binaryName)
	default:
		return nil, fmt.Errorf("unsupported archive format: %s", ext)
	}
}

func extractTarGz(archiveBytes []byte, binaryName string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archiveBytes))
	if err != nil {
		return nil, fmt.Errorf("invalid gzip archive: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	var binaryData []byte
	matchCount := 0

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading tar archive: %w", err)
		}

		cleanName := filepath.Clean(hdr.Name)
		if strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) || strings.HasPrefix(hdr.Name, "/") || strings.HasPrefix(hdr.Name, "\\") {
			return nil, fmt.Errorf("malicious archive entry detected: %s", hdr.Name)
		}

		baseName := filepath.Base(cleanName)
		if baseName == binaryName && (cleanName == binaryName || cleanName == "./"+binaryName || cleanName == filepath.Clean("./"+binaryName)) {
			if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
				return nil, fmt.Errorf("archive entry %s is not a regular file", hdr.Name)
			}
			data, err := io.ReadAll(io.LimitReader(tr, maxArchiveSize))
			if err != nil {
				return nil, fmt.Errorf("error reading archive entry %s: %w", hdr.Name, err)
			}
			binaryData = data
			matchCount++
		}
	}

	if matchCount == 0 {
		return nil, fmt.Errorf("release archive does not contain binary %s", binaryName)
	}
	if matchCount > 1 {
		return nil, fmt.Errorf("release archive contains multiple entries for %s", binaryName)
	}

	return binaryData, nil
}

func extractZip(archiveBytes []byte, binaryName string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(archiveBytes), int64(len(archiveBytes)))
	if err != nil {
		return nil, fmt.Errorf("invalid zip archive: %w", err)
	}

	var binaryData []byte
	matchCount := 0

	for _, f := range zr.File {
		cleanName := filepath.Clean(f.Name)
		if strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) || strings.HasPrefix(f.Name, "/") || strings.HasPrefix(f.Name, "\\") {
			return nil, fmt.Errorf("malicious archive entry detected: %s", f.Name)
		}

		baseName := filepath.Base(cleanName)
		if baseName == binaryName && (cleanName == binaryName || cleanName == "./"+binaryName || cleanName == filepath.Clean("./"+binaryName)) {
			if f.FileInfo().IsDir() {
				return nil, fmt.Errorf("archive entry %s is a directory", f.Name)
			}
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("error opening archive entry %s: %w", f.Name, err)
			}
			data, err := io.ReadAll(io.LimitReader(rc, maxArchiveSize))
			rc.Close()
			if err != nil {
				return nil, fmt.Errorf("error reading archive entry %s: %w", f.Name, err)
			}
			binaryData = data
			matchCount++
		}
	}

	if matchCount == 0 {
		return nil, fmt.Errorf("release archive does not contain binary %s", binaryName)
	}
	if matchCount > 1 {
		return nil, fmt.Errorf("release archive contains multiple entries for %s", binaryName)
	}

	return binaryData, nil
}

// replacementPaths are the files one binary replacement touches: the binary
// and its marker, their staged copies, and the backups a rollback restores.
type replacementPaths struct {
	exePath, markerPath, stagedExe, backupExe, markerBackup, stagedMarker, oldExe string
}

func replaceBinaryWithRollback(ctx context.Context, opts Options, exePath, markerPath string, newBytes []byte, newVersion string) error {
	binDir := filepath.Dir(exePath)
	pid := os.Getpid()
	paths := replacementPaths{
		exePath:      exePath,
		markerPath:   markerPath,
		stagedExe:    filepath.Join(binDir, fmt.Sprintf(".skillstation.new.%d", pid)),
		backupExe:    filepath.Join(binDir, fmt.Sprintf(".skillstation.bak.%d", pid)),
		markerBackup: filepath.Join(binDir, fmt.Sprintf(".skillstation-managed.bak.%d", pid)),
		stagedMarker: filepath.Join(binDir, fmt.Sprintf(".skillstation-managed.new.%d", pid)),
		oldExe:       exePath + ".old",
	}
	stagedExe, backupExe, markerBackup, stagedMarker, oldExe := paths.stagedExe, paths.backupExe, paths.markerBackup, paths.stagedMarker, paths.oldExe

	defer func() {
		_ = os.Remove(stagedExe)
		_ = os.Remove(stagedMarker)
	}()

	if err := stageReplacement(paths, newBytes, newVersion); err != nil {
		return err
	}

	isWindows := opts.GOOS == "windows" || (opts.GOOS == "" && runtime.GOOS == "windows")

	if err := swapBinary(paths, isWindows); err != nil {
		return err
	}

	// Replace marker
	if err := os.Rename(stagedMarker, markerPath); err != nil {
		// Roll back binary and marker
		if isWindows {
			_ = os.Remove(exePath)
			_ = os.Rename(oldExe, exePath)
		} else {
			_ = os.Rename(backupExe, exePath)
			_ = os.Remove(backupExe)
		}
		_ = os.Rename(markerBackup, markerPath)
		return &Error{
			Summary: "Could not update installation marker.",
			Why:     err.Error(),
			Fix:     "Installation was rolled back.",
			Err:     err,
		}
	}

	if verifyErr := verifyReplacedBinary(ctx, opts, exePath); verifyErr != nil {
		// ROLLBACK
		if isWindows {
			_ = os.Remove(exePath)
			_ = os.Rename(oldExe, exePath)
		} else {
			_ = os.Rename(backupExe, exePath)
		}
		_ = os.Rename(markerBackup, markerPath)

		return &Error{
			Summary: "Updated binary failed verification; rolled back to previous version.",
			Why:     fmt.Sprintf("Post-update verification failed: %v", verifyErr),
			Fix:     "The original installation has been restored. Retry the update or report this issue.",
			Err:     verifyErr,
		}
	}

	// Success: clean up backups
	_ = os.Remove(backupExe)
	_ = os.Remove(markerBackup)
	if isWindows {
		_ = os.Remove(oldExe)
	}

	return nil
}

// stageReplacement writes the new binary and marker next to the installed
// ones and backs up the installed marker.
func stageReplacement(paths replacementPaths, newBytes []byte, newVersion string) error {
	// Stage new binary
	if err := os.WriteFile(paths.stagedExe, newBytes, 0755); err != nil {
		return &Error{
			Summary: "Could not write staged binary.",
			Why:     err.Error(),
			Fix:     "Check write permissions in the installation directory.",
			Err:     err,
		}
	}

	// Stage new marker
	newMarker := fmt.Sprintf("%s\nversion=%s\n", ManagedMarkerHeader, newVersion)
	if err := os.WriteFile(paths.stagedMarker, []byte(newMarker), 0600); err != nil {
		return &Error{
			Summary: "Could not write staged installation marker.",
			Why:     err.Error(),
			Fix:     "Check write permissions in the installation directory.",
			Err:     err,
		}
	}

	// Backup existing marker
	if err := copyFile(paths.markerPath, paths.markerBackup); err != nil {
		return &Error{
			Summary: "Could not back up existing installation marker.",
			Why:     err.Error(),
			Fix:     "Check write permissions in the installation directory.",
			Err:     err,
		}
	}
	return nil
}

// swapBinary moves the staged binary into place, keeping the installed one as
// .old on Windows and as a backup copy elsewhere. On failure the installed
// binary is left as it was.
func swapBinary(paths replacementPaths, isWindows bool) error {
	exePath, stagedExe, backupExe, markerBackup, oldExe := paths.exePath, paths.stagedExe, paths.backupExe, paths.markerBackup, paths.oldExe
	if isWindows {
		// On Windows: rename running exe to .old, then staged to exe
		_ = os.Remove(oldExe)
		if err := os.Rename(exePath, oldExe); err != nil {
			_ = os.Remove(markerBackup)
			return &Error{
				Summary: "Could not rename running binary to .old.",
				Why:     err.Error(),
				Fix:     "Ensure no conflicting processes are locking the binary.",
				Err:     err,
			}
		}
		if err := os.Rename(stagedExe, exePath); err != nil {
			// Immediate rollback
			_ = os.Rename(oldExe, exePath)
			_ = os.Remove(markerBackup)
			return &Error{
				Summary: "Could not replace binary with staged binary.",
				Why:     err.Error(),
				Fix:     "The original binary was restored. Retry the update.",
				Err:     err,
			}
		}
		return nil
	}
	// On Unix: copy running exe to backupExe, then atomic rename stagedExe -> exePath
	if err := copyFile(exePath, backupExe); err != nil {
		_ = os.Remove(markerBackup)
		return &Error{
			Summary: "Could not back up running binary.",
			Why:     err.Error(),
			Fix:     "Check write permissions in the installation directory.",
			Err:     err,
		}
	}
	_ = os.Chmod(backupExe, 0755)

	if err := os.Rename(stagedExe, exePath); err != nil {
		_ = os.Remove(backupExe)
		_ = os.Remove(markerBackup)
		return &Error{
			Summary: "Could not atomically replace binary.",
			Why:     err.Error(),
			Fix:     "The original binary was not modified.",
			Err:     err,
		}
	}
	return nil
}

// verifyReplacedBinary runs the new binary's version command, or the test
// verifier when one is set.
func verifyReplacedBinary(ctx context.Context, opts Options, exePath string) error {
	if opts.BinaryVerifier != nil {
		return opts.BinaryVerifier(ctx, exePath)
	}
	cmd := exec.CommandContext(ctx, exePath, "version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// SemVer represents a parsed Semantic Version (SemVer 2.0.0).
type SemVer struct {
	Major      uint64
	Minor      uint64
	Patch      uint64
	Prerelease string
	Build      string
}

// ParseSemVer parses a SemVer string, ignoring any leading 'v'.
func ParseSemVer(s string) (*SemVer, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if s == "" {
		return nil, errors.New("empty version string")
	}

	var build string
	if idx := strings.Index(s, "+"); idx != -1 {
		build = s[idx+1:]
		s = s[:idx]
	}

	var prerelease string
	if idx := strings.Index(s, "-"); idx != -1 {
		prerelease = s[idx+1:]
		s = s[:idx]
	}

	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("expected 3 numeric components, got %d in %q", len(parts), s)
	}

	major, err := parseComponent(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid major: %w", err)
	}
	minor, err := parseComponent(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid minor: %w", err)
	}
	patch, err := parseComponent(parts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid patch: %w", err)
	}

	return &SemVer{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		Prerelease: prerelease,
		Build:      build,
	}, nil
}

func parseComponent(s string) (uint64, error) {
	if s == "" {
		return 0, errors.New("empty component")
	}
	if len(s) > 1 && s[0] == '0' {
		return 0, errors.New("leading zeros not permitted")
	}
	return strconv.ParseUint(s, 10, 64)
}

// Compare returns -1 if a < b, 0 if a == b, and 1 if a > b.
func (a *SemVer) Compare(b *SemVer) int {
	if a.Major != b.Major {
		if a.Major < b.Major {
			return -1
		}
		return 1
	}
	if a.Minor != b.Minor {
		if a.Minor < b.Minor {
			return -1
		}
		return 1
	}
	if a.Patch != b.Patch {
		if a.Patch < b.Patch {
			return -1
		}
		return 1
	}

	if a.Prerelease == "" && b.Prerelease != "" {
		return 1
	}
	if a.Prerelease != "" && b.Prerelease == "" {
		return -1
	}
	if a.Prerelease == "" && b.Prerelease == "" {
		return 0
	}

	return comparePrereleases(a.Prerelease, b.Prerelease)
}

func comparePrereleases(preA, preB string) int {
	partsA := strings.Split(preA, ".")
	partsB := strings.Split(preB, ".")
	minLen := len(partsA)
	if len(partsB) < minLen {
		minLen = len(partsB)
	}

	for i := range minLen {
		idA, idB := partsA[i], partsB[i]
		if idA == idB {
			continue
		}
		numA, isNumA := parseUint(idA)
		numB, isNumB := parseUint(idB)

		if isNumA && isNumB {
			if numA < numB {
				return -1
			}
			return 1
		}
		if isNumA && !isNumB {
			return -1
		}
		if !isNumA && isNumB {
			return 1
		}
		if idA < idB {
			return -1
		}
		return 1
	}

	if len(partsA) < len(partsB) {
		return -1
	}
	if len(partsA) > len(partsB) {
		return 1
	}
	return 0
}

func parseUint(s string) (uint64, bool) {
	if s == "" {
		return 0, false
	}
	n, err := strconv.ParseUint(s, 10, 64)
	return n, err == nil
}

// CompareVersions compares two version strings. Returns (-1, 0, 1) and true if both
// parsed as SemVer. Otherwise falls back to string/dev comparison and returns false.
func CompareVersions(current, target string) (int, bool) {
	svCur, errCur := ParseSemVer(current)
	svTar, errTar := ParseSemVer(target)
	if errCur == nil && errTar == nil {
		return svCur.Compare(svTar), true
	}
	if current == target {
		return 0, false
	}
	if errCur != nil && errTar == nil {
		// e.g. current is "dev", target is "0.2.0": current is older
		return -1, false
	}
	if errCur == nil && errTar != nil {
		return 1, false
	}
	if current < target {
		return -1, false
	}
	return 1, false
}
