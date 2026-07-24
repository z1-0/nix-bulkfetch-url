package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
)

func download(ctx context.Context, url, dest string, onProgress progressFunc) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("downloading: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	total := resp.ContentLength
	if total < 0 {
		total = 0
	}
	if onProgress != nil {
		onProgress(0, total)
	}

	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()

	pr := &progressReader{
		r:          resp.Body,
		onProgress: onProgress,
		total:      total,
	}

	_, err = io.Copy(f, pr)
	if err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	return nil
}
