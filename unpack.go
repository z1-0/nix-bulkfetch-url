package main

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type archiveFormat struct {
	extensions []string
	args       []string
	destFlag   string
}

var archiveFormats = map[string]archiveFormat{
	"tar.gz":   {[]string{".tar.gz", ".tgz"}, []string{"tar", "xzf"}, "-C"},
	"tar.xz":   {[]string{".tar.xz", ".txz"}, []string{"tar", "xJf"}, "-C"},
	"tar.bz2":  {[]string{".tar.bz2", ".tbz2"}, []string{"tar", "xjf"}, "-C"},
	"tar.zst":  {[]string{".tar.zst", ".tzst"}, []string{"tar", "--zstd", "-xf"}, "-C"},
	"tar.lzma": {[]string{".tar.lzma", ".tlzma"}, []string{"tar", "--lzma", "-xf"}, "-C"},
	"tar.lz":   {[]string{".tar.lz", ".tlz"}, []string{"tar", "--lzip", "-xf"}, "-C"},
	"tar.lz4":  {[]string{".tar.lz4", ".tlz4"}, []string{"tar", "--lz4", "-xf"}, "-C"},
	"tar.lzo":  {[]string{".tar.lzo", ".tlzo"}, []string{"tar", "--lzo", "-xf"}, "-C"},
	"tar.Z":    {[]string{".tar.Z", ".tZ"}, []string{"tar", "-Z", "-xf"}, "-C"},
	"zip":      {[]string{".zip"}, []string{"unzip", "-q"}, "-d"},
}

func unpackWithSource(archivePath, destDir, sourceURL string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("creating dest dir: %w", err)
	}

	format := detectFormat(sourceURL)
	if format == "" {
		return fmt.Errorf("unsupported archive format: %s", filepath.Base(sourceURL))
	}

	af, ok := archiveFormats[format]
	if !ok {
		return fmt.Errorf("unsupported archive format: %s", format)
	}

	return run(af.args[0], append(af.args[1:], archivePath, af.destFlag, destDir)...)
}

func detectFormat(sourceURL string) string {
	u, err := url.Parse(sourceURL)
	if err != nil {
		return ""
	}
	return matchExt(strings.ToLower(u.Path))
}

func matchExt(path string) string {
	for format, af := range archiveFormats {
		for _, ext := range af.extensions {
			if strings.HasSuffix(path, ext) {
				return format
			}
		}
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
