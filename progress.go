package main

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type workerState struct {
	mu         sync.Mutex
	url        string
	downloaded int64
	total      int64
	err        string
}

func (ws *workerState) start(url string) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ws.url = url
	ws.downloaded = 0
	ws.total = 0
	ws.err = ""
}

func (ws *workerState) update(downloaded, total int64) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ws.downloaded = downloaded
	if total > 0 {
		ws.total = total
	}
}

func (ws *workerState) fail(reason string) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ws.err = reason
}

func (ws *workerState) done() {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ws.url = ""
	ws.err = ""
	ws.downloaded = 0
	ws.total = 0
}

func (ws *workerState) snapshot() workerState {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	return workerState{
		url:        ws.url,
		downloaded: ws.downloaded,
		total:      ws.total,
		err:        ws.err,
	}
}

type progressDisplay struct {
	workers   []*workerState
	completed atomic.Int64
	total     int
	tty       bool
	rows      int
	stopCh    chan struct{}
	w         io.Writer
	width     int
}

func newProgressDisplay(numWorkers, numURLs int) *progressDisplay {
	dispWorkers := roundUpWorkers(numWorkers)
	rows := calcRows(numWorkers)
	ws := make([]*workerState, dispWorkers)
	for i := range ws {
		ws[i] = &workerState{}
	}
	fi, err := os.Stderr.Stat()
	tty := err == nil && (fi.Mode()&os.ModeCharDevice) != 0
	return &progressDisplay{
		workers: ws,
		total:   numURLs,
		tty:     tty,
		rows:    rows,
		stopCh:  make(chan struct{}),
		w:       os.Stderr,
		width:   termWidth(),
	}
}

func termWidth() int {
	if w, _, err := termSize(os.Stderr.Fd()); err == nil && w > 0 {
		return w
	}
	return 80
}

func calcRows(workers int) int {
	return roundUpWorkers(workers) / 2
}

func roundUpWorkers(n int) int {
	if n > 16 {
		n = 16
	}
	if n%2 != 0 {
		n++
	}
	return n
}

func formatBytes(n int64) string {
	if n < 1024 {
		return strconv.FormatInt(n, 10) + " B"
	}
	f := float64(n)
	switch {
	case f < math.Pow(1024, 2):
		return fmt.Sprintf("%.1f KiB", f/1024)
	case f < math.Pow(1024, 3):
		return fmt.Sprintf("%.1f MiB", f/(1024*1024))
	default:
		return fmt.Sprintf("%.1f GiB", f/(1024*1024*1024))
	}
}

func formatProgress(downloaded, total int64) string {
	if total > 0 {
		return "[" + formatBytes(downloaded) + "/" + formatBytes(total) + "]"
	}
	return "[" + formatBytes(downloaded) + "]"
}

func formatWorkerLine(ws *workerState, halfWidth int) string {
	s := ws.snapshot()
	if s.err != "" {
		msg := "[FAIL] " + s.err
		if len([]rune(msg)) > halfWidth {
			msg = string([]rune(msg)[:halfWidth-3]) + "..."
		}
		return msg
	}
	if s.url == "" {
		return ""
	}
	line := fmt.Sprintf("%s '%s'", formatProgress(s.downloaded, s.total), s.url)
	if len([]rune(line)) > halfWidth {
		line = string([]rune(line)[:halfWidth-3]) + "..."
	}
	return line
}

func (pd *progressDisplay) workerByIndex(idx int) *workerState {
	if idx < 0 || idx >= len(pd.workers) {
		return nil
	}
	return pd.workers[idx]
}

func (pd *progressDisplay) start(ctx context.Context) {
	if !pd.tty || pd.rows == 0 {
		return
	}
	pd.allocateLines()
	go pd.renderLoop(ctx)
}

func (pd *progressDisplay) allocateLines() {
	for i := 0; i < pd.rows+1; i++ {
		fmt.Fprintln(pd.w)
	}
	fmt.Fprintf(pd.w, "\033[%dA", pd.rows+1)
}

func (pd *progressDisplay) renderLoop(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	pd.render()
	for {
		select {
		case <-ctx.Done():
			return
		case <-pd.stopCh:
			pd.renderFinal()
			return
		case <-ticker.C:
			pd.render()
		}
	}
}

func (pd *progressDisplay) render() {
	if !pd.tty || pd.rows == 0 {
		return
	}
	half := pd.width / 2
	var b strings.Builder
	b.Grow(pd.width * (pd.rows + 1))
	b.WriteString(fmt.Sprintf("\033[%dA", pd.rows))
	for r := 0; r < pd.rows; r++ {
		b.WriteString("\033[K")
		left := formatWorkerLine(pd.workers[r*2], half)
		right := formatWorkerLine(pd.workers[r*2+1], half)
		if left != "" {
			b.WriteString(left)
		}
		if left != "" && right != "" {
			pad := half - len([]rune(left))
			if pad < 1 {
				pad = 1
			}
			if pad+len([]rune(right)) >= half {
				pad = half - len([]rune(right))
				if pad < 1 {
					pad = 1
				}
			}
			b.WriteString(strings.Repeat(" ", pad))
		} else if right != "" {
			b.WriteString(strings.Repeat(" ", half))
		}
		if right != "" {
			b.WriteString(right)
		}
		if r < pd.rows-1 {
			b.WriteString("\n")
		}
	}
	b.WriteString("\n\033[K")
	c := pd.completed.Load()
	b.WriteString(fmt.Sprintf("[%d/%d]", c, pd.total))
	pd.w.Write([]byte(b.String()))
}

func (pd *progressDisplay) renderFinal() {
	if !pd.tty || pd.rows == 0 {
		return
	}
	pd.clearAll()
	c := pd.total
	if cVal := pd.completed.Load(); cVal > 0 {
		c = int(cVal)
	}
	fmt.Fprintf(pd.w, "[%d/%d] done\n", c, pd.total)
}

func (pd *progressDisplay) clearAll() {
	if !pd.tty || pd.rows == 0 {
		return
	}
	fmt.Fprintf(pd.w, "\033[%dA", pd.rows)
	for r := 0; r <= pd.rows; r++ {
		pd.w.Write([]byte("\033[K"))
		if r < pd.rows {
			pd.w.Write([]byte("\n"))
		}
	}
	fmt.Fprintf(pd.w, "\033[%dA\033[J", pd.rows+1)
}

func (pd *progressDisplay) stop() {
	close(pd.stopCh)
}

type progressFunc func(downloaded, total int64)

type progressReader struct {
	r          io.Reader
	onProgress func(downloaded, total int64)
	downloaded int64
	total      int64
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.r.Read(p)
	if n > 0 {
		pr.downloaded += int64(n)
		pr.onProgress(pr.downloaded, pr.total)
	}
	return n, err
}

// overridden in tests
var termSize = func(fd uintptr) (int, int, error) {
	return 80, 24, nil
}
