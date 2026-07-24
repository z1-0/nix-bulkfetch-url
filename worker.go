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
	Format   string
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

	display := newProgressDisplay(opts.Workers, total)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	display.start(ctx)

	numWorkers := opts.Workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range ch {
				select {
				case <-ctx.Done():
					continue
				default:
				}

				ws := display.workerForURL(idx)
				if ws != nil {
					ws.start(urls[idx])
				}
				result := processURL(ctx, urls[idx], idx, opts, ws)
				display.completed.Add(1)
				if ws != nil {
					ws.done()
				}

				mu.Lock()
				results[idx] = result
				if opts.FailFast && result.Error != nil {
					cancel()
				}
				mu.Unlock()
			}
		}()
	}

	for i := range urls {
		ch <- i
	}
	close(ch)

	wg.Wait()
	display.stop()
	return results
}

func processURL(ctx context.Context, url string, index int, opts Options, ws *workerState) Result {
	var lastErr error

	for attempt := 0; attempt <= opts.Retries; attempt++ {
		if ctx.Err() != nil {
			return Result{Index: index, URL: url, Error: ctx.Err()}
		}

		if attempt > 0 {
			timer := time.NewTimer(time.Duration(1<<(attempt-1)) * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return Result{Index: index, URL: url, Error: ctx.Err()}
			case <-timer.C:
			}
		}

		result, err := tryFetch(ctx, url, opts, ws)
		if err == nil {
			return Result{Index: index, URL: url, Hash: result}
		}
		lastErr = err
	}

	if ws != nil {
		ws.fail(lastErr.Error())
	}
	return Result{Index: index, URL: url, Error: lastErr}
}

func tryFetch(ctx context.Context, url string, opts Options, ws *workerState) (string, error) {
	tmpDir, err := os.MkdirTemp("", "nix-bulkfetch-*")
	if err != nil {
		return "", fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	dlCtx, cancel := context.WithTimeout(ctx, time.Duration(opts.Timeout)*time.Second)
	defer cancel()

	var onProgress progressFunc
	if ws != nil {
		onProgress = func(downloaded, total int64) {
			ws.update(downloaded, total)
		}
	}

	archivePath := filepath.Join(tmpDir, "download")
	if err := download(dlCtx, url, archivePath, onProgress); err != nil {
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

	hash, err := nixHash(opts.HashType, opts.Format, hashPath, !opts.Unpack)
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
