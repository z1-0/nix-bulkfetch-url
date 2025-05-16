package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Options struct {
	Workers  int
	HashType string
	Unpack   bool
	Timeout  int
	Retries  int
	FailFast bool
}

type Result struct {
	Index int
	URL   string
	Hash  string
	Error error
}

func WorkerPool(urls []string, opts Options) []Result {
	results := make([]Result, len(urls))
	ch := make(chan int, len(urls))
	var wg sync.WaitGroup
	var mu sync.Mutex

	total := len(urls)
	completed := 0

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for i := 0; i < opts.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range ch {
				select {
				case <-ctx.Done():
					continue
				default:
				}

				result := processURL(ctx, urls[idx], idx, opts)

				mu.Lock()
				results[idx] = result
				completed++
				if opts.FailFast && result.Error != nil {
					cancel()
				}
				fmt.Fprintf(os.Stderr, "[%d/%d] %s\n", completed, total, urls[idx])
				mu.Unlock()
			}
		}()
	}

	for i := range urls {
		ch <- i
	}
	close(ch)

	wg.Wait()
	return results
}

func processURL(ctx context.Context, url string, index int, opts Options) Result {
	var lastErr error

	for attempt := 0; attempt <= opts.Retries; attempt++ {
		select {
		case <-ctx.Done():
			return Result{Index: index, URL: url, Error: ctx.Err()}
		default:
		}

		if attempt > 0 {
			time.Sleep(time.Duration(1<<(attempt-1)) * time.Second)
		}

		result, err := tryFetch(ctx, url, opts)
		if err == nil {
			return Result{Index: index, URL: url, Hash: result}
		}
		lastErr = err
	}

	return Result{Index: index, URL: url, Error: lastErr}
}

func tryFetch(ctx context.Context, url string, opts Options) (string, error) {
	tmpDir, err := os.MkdirTemp("", "nix-bulkfetch-*")
	if err != nil {
		return "", fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	dlCtx, cancel := context.WithTimeout(ctx, time.Duration(opts.Timeout)*time.Second)
	defer cancel()

	archivePath := filepath.Join(tmpDir, "download")
	if err := download(dlCtx, url, archivePath); err != nil {
		return "", fmt.Errorf("downloading: %w", err)
	}

	hashPath := archivePath

	if opts.Unpack {
		unpackDir := filepath.Join(tmpDir, "unpacked")
		if err := unpackWithSource(archivePath, unpackDir, url); err != nil {
			return "", fmt.Errorf("unpacking: %w", err)
		}
		hashPath = findUnpackedDir(unpackDir)
	}

	hash, err := nixHash(opts.HashType, hashPath, !opts.Unpack)
	if err != nil {
		return "", fmt.Errorf("hashing: %w", err)
	}

	return hash, nil
}

func findUnpackedDir(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return dir
	}

	if len(entries) == 1 && entries[0].IsDir() {
		return filepath.Join(dir, entries[0].Name())
	}

	return dir
}
