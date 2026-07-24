package main

import (
	"testing"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1023, "1023 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1048576, "1.0 MiB"},
		{1572864, "1.5 MiB"},
		{1073741824, "1.0 GiB"},
		{1610612736, "1.5 GiB"},
		{209715200, "200.0 MiB"},
	}
	for _, tt := range tests {
		got := formatBytes(tt.n)
		if got != tt.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestFormatProgress(t *testing.T) {
	tests := []struct {
		downloaded int64
		total      int64
		want       string
	}{
		{0, 0, "[0 B]"},
		{500, 1024, "[500 B/1.0 KiB]"},
		{0, 1048576, "[0 B/1.0 MiB]"},
		{524288, 1048576, "[512.0 KiB/1.0 MiB]"},
	}
	for _, tt := range tests {
		got := formatProgress(tt.downloaded, tt.total)
		if got != tt.want {
			t.Errorf("formatProgress(%d,%d) = %q, want %q", tt.downloaded, tt.total, got, tt.want)
		}
	}
}

func TestCalcRows(t *testing.T) {
	tests := []struct {
		workers int
		want    int
	}{
		{0, 0},
		{1, 1},
		{2, 1},
		{3, 2},
		{4, 2},
		{5, 3},
		{6, 3},
		{15, 8},
		{16, 8},
		{17, 8},
		{100, 8},
	}
	for _, tt := range tests {
		got := calcRows(tt.workers)
		if got != tt.want {
			t.Errorf("calcRows(%d) = %d, want %d", tt.workers, got, tt.want)
		}
	}
}

func TestMakeDisplayWorkers(t *testing.T) {
	p := newProgressDisplay(5, 10)
	if len(p.workers) != 6 {
		t.Fatalf("expected 6 worker slots, got %d", len(p.workers))
	}
	if p.completed.Load() != 0 {
		t.Errorf("expected completed=0, got %d", p.completed.Load())
	}
	if p.total != 10 {
		t.Errorf("expected total=10, got %d", p.total)
	}
}

func TestWorkerStateStart(t *testing.T) {
	ws := &workerState{}
	ws.start("https://example.com/file")
	if ws.url != "https://example.com/file" {
		t.Errorf("url = %q, want %q", ws.url, "https://example.com/file")
	}
	if ws.downloaded != 0 {
		t.Errorf("downloaded = %d, want 0", ws.downloaded)
	}
	if ws.total != 0 {
		t.Errorf("total = %d, want 0", ws.total)
	}
	if ws.err != "" {
		t.Errorf("err = %q, want empty", ws.err)
	}
}

func TestWorkerStateUpdate(t *testing.T) {
	ws := &workerState{}
	ws.start("https://example.com/file")
	ws.update(524288, 1048576)
	if ws.downloaded != 524288 {
		t.Errorf("downloaded = %d, want 524288", ws.downloaded)
	}
	if ws.total != 1048576 {
		t.Errorf("total = %d, want 1048576", ws.total)
	}
}

func TestWorkerStateFail(t *testing.T) {
	ws := &workerState{}
	ws.start("https://example.com/file")
	ws.fail("connection timeout")
	if ws.err != "connection timeout" {
		t.Errorf("err = %q, want %q", ws.err, "connection timeout")
	}
}

func TestWorkerStateDone(t *testing.T) {
	ws := &workerState{}
	ws.start("https://example.com/file")
	ws.update(1048576, 1048576)
	ws.done()
	snap := ws.snapshot()
	if snap.url != "" {
		t.Errorf("url after done = %q, want empty", snap.url)
	}
	if snap.err != "" {
		t.Errorf("err after done = %q, want empty", snap.err)
	}
	if snap.downloaded != 0 {
		t.Errorf("downloaded after done = %d, want 0", snap.downloaded)
	}
}

func TestFormatWorkerLine(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		downloaded int64
		total     int64
		err       string
		halfWidth int
		want      string
	}{
		{
			name:       "empty worker",
			halfWidth:  40,
			want:       "",
		},
		{
			name:       "active download full visible",
			url:        "https://example.com/file.tar.gz",
			downloaded: 524288,
			total:      1048576,
			halfWidth:  60,
			want:       "[512.0 KiB/1.0 MiB] 'https://example.com/file.tar.gz'",
		},
		{
			name:       "truncated url",
			url:        "https://very-long-domain-name.example.com/path/to/some/file.tar.gz",
			downloaded: 1048576,
			total:      2097152,
			halfWidth:  40,
			want:       "[1.0 MiB/2.0 MiB] 'https://very-long-...",
		},
		{
			name:       "failed",
			url:        "https://example.com/file.tar.gz",
			err:        "connection refused",
			halfWidth:  40,
			want:       "[FAIL] connection refused",
		},
		{
			name:       "truncated fail message",
			url:        "https://example.com/file.tar.gz",
			err:        "connection refused: dial tcp 192.168.1.1:443: i/o timeout",
			halfWidth:  30,
			want:       "[FAIL] connection refused: ...",
		},
		{
			name:       "unknown total",
			url:        "https://example.com/stream",
			downloaded: 1048576,
			total:      0,
			halfWidth:  40,
			want:       "[1.0 MiB] 'https://example.com/stream'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws := &workerState{}
			if tt.url != "" {
				ws.start(tt.url)
				ws.update(tt.downloaded, tt.total)
				if tt.err != "" {
					ws.fail(tt.err)
				}
			}
			got := formatWorkerLine(ws, tt.halfWidth)
			if got != tt.want {
				t.Errorf("formatWorkerLine = %q, want %q", got, tt.want)
			}
		})
	}
}
