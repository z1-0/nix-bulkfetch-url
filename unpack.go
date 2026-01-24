package main

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func unpackWithSource(archivePath, destDir, sourceURL string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("creating dest dir: %w", err)
	}

	format := detectFormat(sourceURL)
	if format == "" {
		return fmt.Errorf("unsupported archive format: %s", filepath.Base(sourceURL))
	}

	switch format {
	case "tar.gz":
		return run("tar", "xzf", archivePath, "-C", destDir)
	case "tar.xz":
		return run("tar", "xJf", archivePath, "-C", destDir)
	case "tar.bz2":
		return run("tar", "xjf", archivePath, "-C", destDir)
	case "tar.zst":
		return run("tar", "--zstd", "-xf", archivePath, "-C", destDir)
	case "tar.lzma":
		return run("tar", "--lzma", "-xf", archivePath, "-C", destDir)
	case "tar.lz":
		return run("tar", "--lzip", "-xf", archivePath, "-C", destDir)
	case "tar.lz4":
		return run("tar", "--lz4", "-xf", archivePath, "-C", destDir)
	case "tar.lzo":
		return run("tar", "--lzo", "-xf", archivePath, "-C", destDir)
	case "tar.Z":
		return run("tar", "-Z", "-xf", archivePath, "-C", destDir)
	case "zip":
		return run("unzip", "-q", archivePath, "-d", destDir)
	}
	return nil
}

func detectFormat(sourceURL string) string {
	u, err := url.Parse(sourceURL)
	if err != nil {
		return ""
	}
	return matchExt(strings.ToLower(u.Path))
}

func matchExt(path string) string {
	switch {
	case strings.HasSuffix(path, ".tar.gz") || strings.HasSuffix(path, ".tgz"):
		return "tar.gz"
	case strings.HasSuffix(path, ".tar.xz") || strings.HasSuffix(path, ".txz"):
		return "tar.xz"
	case strings.HasSuffix(path, ".tar.bz2") || strings.HasSuffix(path, ".tbz2"):
		return "tar.bz2"
	case strings.HasSuffix(path, ".tar.zst") || strings.HasSuffix(path, ".tzst"):
		return "tar.zst"
	case strings.HasSuffix(path, ".tar.lzma") || strings.HasSuffix(path, ".tlzma"):
		return "tar.lzma"
	case strings.HasSuffix(path, ".tar.lz") || strings.HasSuffix(path, ".tlz"):
		return "tar.lz"
	case strings.HasSuffix(path, ".tar.lz4") || strings.HasSuffix(path, ".tlz4"):
		return "tar.lz4"
	case strings.HasSuffix(path, ".tar.lzo") || strings.HasSuffix(path, ".tlzo"):
		return "tar.lzo"
	case strings.HasSuffix(path, ".tar.Z") || strings.HasSuffix(path, ".tZ"):
		return "tar.Z"
	case strings.HasSuffix(path, ".zip"):
		return "zip"
	}
	return ""
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s failed: %w\noutput: %s", name, err, string(out))
	}
	return nil
}
