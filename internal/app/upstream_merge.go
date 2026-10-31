package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

const maxMergeableTextBytes = 256 * 1024

func mergeableText(content []byte) bool {
	if len(content) > maxMergeableTextBytes {
		return false
	}
	if bytes.Contains(content, []byte{0}) {
		return false
	}
	return utf8.Valid(content)
}

func gitMergeFile(ctx context.Context, root string, base, local, upstream []byte) ([]byte, int, error) {
	dir, cleanup, err := newRuntimeTemp(root, "upstream-merge-")
	if err != nil {
		return nil, 0, err
	}
	defer cleanup()

	baseFile := filepath.Join(dir, "base")
	localFile := filepath.Join(dir, "local")
	upstreamFile := filepath.Join(dir, "upstream")

	if err := os.WriteFile(baseFile, base, 0o600); err != nil {
		return nil, 0, err
	}
	if err := os.WriteFile(localFile, local, 0o600); err != nil {
		return nil, 0, err
	}
	if err := os.WriteFile(upstreamFile, upstream, 0o600); err != nil {
		return nil, 0, err
	}

	opCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := offlineGitCommand(opCtx, root, "merge-file", "-p", "--diff3", "-L", "local", "-L", "base", "-L", "upstream", localFile, baseFile, upstreamFile)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	if runErr == nil {
		return stdout.Bytes(), 0, nil
	}

	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		code := exitErr.ExitCode()
		if code >= 1 && code <= 127 {
			return stdout.Bytes(), code, nil
		}
		return nil, 0, fmt.Errorf("git merge-file failed (exit %d): %s", code, strings.TrimSpace(stderr.String()))
	}
	return nil, 0, fmt.Errorf("git merge-file failed: %w", runErr)
}

func gitDiffNoIndex(ctx context.Context, root, targetPath string, from, to []byte) (string, error) {
	if len(from) == 0 && len(to) == 0 {
		return "", nil
	}

	dir, cleanup, err := newRuntimeTemp(root, "upstream-merge-")
	if err != nil {
		return "", err
	}
	defer cleanup()

	fromFile := filepath.Join(dir, "from")
	toFile := filepath.Join(dir, "to")

	fromBytes := from
	if fromBytes == nil {
		fromBytes = []byte{}
	}
	toBytes := to
	if toBytes == nil {
		toBytes = []byte{}
	}

	if err := os.WriteFile(fromFile, fromBytes, 0o600); err != nil {
		return "", err
	}
	if err := os.WriteFile(toFile, toBytes, 0o600); err != nil {
		return "", err
	}

	opCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := offlineGitCommand(opCtx, root, "diff", "--no-index", "--no-color", "--no-ext-diff", "-U3", fromFile, toFile)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	if runErr == nil {
		return "", nil
	}

	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		if exitErr.ExitCode() != 1 {
			return "", fmt.Errorf("git diff --no-index failed (exit %d): %s", exitErr.ExitCode(), strings.TrimSpace(stderr.String()))
		}
	} else {
		return "", fmt.Errorf("git diff --no-index failed: %w", runErr)
	}

	cleanPath := filepath.ToSlash(targetPath)
	fromPath := "a/" + cleanPath
	if len(from) == 0 {
		fromPath = "/dev/null"
	}
	toPath := "b/" + cleanPath
	if len(to) == 0 {
		toPath = "/dev/null"
	}

	lines := strings.Split(stdout.String(), "\n")
	var out []string
	for i, line := range lines {
		switch {
		case i == 0 && strings.HasPrefix(line, "diff --git "):
			out = append(out, fmt.Sprintf("diff --git a/%s b/%s", cleanPath, cleanPath))
		case strings.HasPrefix(line, "--- "):
			out = append(out, "--- "+fromPath)
		case strings.HasPrefix(line, "+++ "):
			out = append(out, "+++ "+toPath)
		default:
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n"), nil
}
