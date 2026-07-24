package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

const (
	kib = 1024
	mib = kib * 1024
	gib = mib * 1024
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
	total     int64
	tty       bool
	rows      int
	stopCh    chan struct{}
	doneCh    chan struct{}
	w         io.Writer
	width     int
	started   bool
}

func newProgressDisplay(numWorkers, numURLs int) *progressDisplay {
	displaySlots := numWorkers
	if numURLs < displaySlots {
		displaySlots = numURLs
	}
	displaySlots = roundUpWorkers(displaySlots)
	if displaySlots%2 != 0 {
		displaySlots++
	}
	rows := displaySlots / 2
	ws := make([]*workerState, displaySlots)
	for i := range ws {
		ws[i] = &workerState{}
	}
	fi, err := os.Stderr.Stat()
	tty := err == nil && (fi.Mode()&os.ModeCharDevice) != 0
	return &progressDisplay{
		workers: ws,
		total:   int64(numURLs),
		tty:     tty,
		rows:    rows,
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
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
	if n < kib {
		return strconv.FormatInt(n, 10) + " B"
	}
	f := float64(n)
	switch {
	case f < mib:
		return fmt.Sprintf("%.1f KiB", f/kib)
	case f < gib:
		return fmt.Sprintf("%.1f MiB", f/mib)
	default:
		return fmt.Sprintf("%.1f GiB", f/gib)
	}
}

func formatProgress(downloaded, total int64) string {
	if total > 0 {
		return "[" + formatBytes(downloaded) + "/" + formatBytes(total) + "]"
	}
	return "[" + formatBytes(downloaded) + "]"
}

func colorizeProgress(prog string) string {
	if len(prog) < 3 {
		return prog
	}
	inner := prog[1 : len(prog)-1]
	if slash := strings.IndexByte(inner, '/'); slash >= 0 {
		x1 := inner[:slash]
		x2 := inner[slash:]
		return "\033[32m[" + x1 + "\033[0m" + x2 + "\033[32m]\033[0m"
	}
	return "\033[32m[" + inner + "]\033[0m"
}

func formatWorkerLine(ws *workerState, halfWidth int) string {
	s := ws.snapshot()
	if s.err != "" {
		visible := "[FAIL] " + s.err
		if len([]rune(visible)) > halfWidth {
			visible = string([]rune(visible)[:halfWidth-3]) + "..."
		}
		return "\033[31m" + visible[:6] + "\033[0m" + visible[6:]
	}
	if s.url == "" {
		return ""
	}
	prog := formatProgress(s.downloaded, s.total)
	visible := prog + " " + s.url
	if len([]rune(visible)) > halfWidth {
		visible = string([]rune(visible)[:halfWidth-3]) + "..."
	}
	progLen := len([]rune(prog))
	if progLen > len([]rune(visible)) {
		progLen = len([]rune(visible))
	}
	runes := []rune(visible)
	return colorizeProgress(string(runes[:progLen])) + string(runes[progLen:])
}

func visibleLen(s string) int {
	var n int
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '\033' {
			for i < len(runes) && runes[i] != 'm' {
				i++
			}
			continue
		}
		n++
	}
	return n
}

func (pd *progressDisplay) workerForURL(urlIdx int) *workerState {
	if len(pd.workers) == 0 {
		return nil
	}
	return pd.workers[urlIdx%len(pd.workers)]
}

func (pd *progressDisplay) start(ctx context.Context) {
	if !pd.tty || pd.rows == 0 {
		return
	}
	pd.allocateLines()
	go pd.renderLoop(ctx)
}

func (pd *progressDisplay) allocateLines() {
	for i := 0; i < pd.rows+2; i++ {
		fmt.Fprintln(pd.w)
	}
	fmt.Fprintf(pd.w, "\033[%dA", pd.rows+2)
}

func (pd *progressDisplay) renderLoop(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	pd.render()
	for {
		select {
		case <-ctx.Done():
			close(pd.doneCh)
			return
		case <-pd.stopCh:
			pd.renderFinal()
			close(pd.doneCh)
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
	colWidth := (pd.width - 1) / 2
	if colWidth < 5 {
		colWidth = 5
	}
	var b strings.Builder
	b.Grow(pd.width * (pd.rows + 1))
	if pd.started {
		b.WriteString(fmt.Sprintf("\033[%dA", pd.rows+1))
	}
	pd.started = true
	for r := 0; r < pd.rows; r++ {
		b.WriteString("\r\033[K")
		left := formatWorkerLine(pd.workers[r*2], colWidth)
		right := formatWorkerLine(pd.workers[r*2+1], colWidth)
		if left != "" && right != "" {
			leftVis := visibleLen(left)
			if leftVis > colWidth {
				leftVis = colWidth
			}
			gap := colWidth + 1 - leftVis
			if gap < 1 {
				gap = 1
			}
			b.WriteString(left)
			b.WriteString(strings.Repeat(" ", gap))
			b.WriteString(right)
		} else if right != "" {
			b.WriteString(strings.Repeat(" ", colWidth+1))
			b.WriteString(right)
		} else {
			b.WriteString(left)
		}
		if r < pd.rows-1 {
			b.WriteString("\n")
		}
	}
	b.WriteString("\n\n\r\033[K")
	c := pd.completed.Load()
	b.WriteString(fmt.Sprintf("\033[32m[%d/%d]\033[0m", c, pd.total))
	pd.w.Write([]byte(b.String()))
}

func (pd *progressDisplay) renderFinal() {
	if !pd.tty || pd.rows == 0 {
		return
	}
	// cursor is at global line after last render
	for r := 0; r < pd.rows+1; r++ {
		fmt.Fprintf(pd.w, "\033[A\r\033[K")
	}
	fmt.Fprintf(pd.w, "\r\033[K")
	c := pd.total
	if cVal := pd.completed.Load(); cVal > 0 {
		c = cVal
	}
	fmt.Fprintf(pd.w, "\r\033[32m[%d/%d]\033[0m done\n\n\033[J", c, pd.total)
}

func (pd *progressDisplay) stop() {
	if !pd.tty || pd.rows == 0 {
		return
	}
	close(pd.stopCh)
	<-pd.doneCh
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

var termSize = func(fd uintptr) (int, int, error) {
	var ws struct{ Row, Col, Xpixel, Ypixel uint16 }
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		fd,
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(&ws)),
	)
	if errno != 0 {
		return 0, 0, errno
	}
	return int(ws.Col), int(ws.Row), nil
}
