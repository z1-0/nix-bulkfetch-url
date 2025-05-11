package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func unpackWithSource(archivePath, destDir, sourceURL string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("creating dest dir: %w", err)
	}

	lower := strings.ToLower(sourceURL)
	if idx := strings.Index(lower, "?"); idx != -1 {
		lower = lower[:idx]
	}

	switch {
	case strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz"):
		return unpackTarGz(archivePath, destDir)
	case strings.HasSuffix(lower, ".tar.xz") || strings.HasSuffix(lower, ".txz"):
		return unpackTarXz(archivePath, destDir)
	case strings.HasSuffix(lower, ".zip"):
		return unpackZip(archivePath, destDir)
	default:
		u, err := url.Parse(sourceURL)
		if err != nil {
			return fmt.Errorf("unsupported archive format: %s", filepath.Base(sourceURL))
		}
		path := strings.ToLower(u.Path)
		switch {
		case strings.HasSuffix(path, ".tar.gz") || strings.HasSuffix(path, ".tgz"):
			return unpackTarGz(archivePath, destDir)
		case strings.HasSuffix(path, ".tar.xz") || strings.HasSuffix(path, ".txz"):
			return unpackTarXz(archivePath, destDir)
		case strings.HasSuffix(path, ".zip"):
			return unpackZip(archivePath, destDir)
		default:
			return fmt.Errorf("unsupported archive format: %s", filepath.Base(sourceURL))
		}
	}
}

func unpackTarGz(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()

	return unpackTar(gz, destDir)
}

func unpackTarXz(archivePath, destDir string) error {
	return fmt.Errorf("tar.xz not implemented yet, use tar.gz or zip")
}

func unpackTar(r io.Reader, destDir string) error {
	tr := tar.NewReader(r)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar reader: %w", err)
		}

		target := filepath.Join(destDir, header.Name)

		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid path in archive: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			mode := os.FileMode(header.Mode)
			if err := os.MkdirAll(target, mode); err != nil {
				return err
			}
			if err := os.Chmod(target, mode); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			mode := os.FileMode(header.Mode)
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
			if err := os.Chmod(target, mode); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			os.Remove(target)
			if err := os.Symlink(header.Linkname, target); err != nil {
				return err
			}
		}
	}
	return nil
}

func unpackZip(archivePath, destDir string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		target := filepath.Join(destDir, f.Name)

		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid path in archive: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
