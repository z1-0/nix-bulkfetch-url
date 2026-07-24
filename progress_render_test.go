package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestRenderFourWorkersTwoRows(t *testing.T) {
	var buf bytes.Buffer
	d := newTestDisplay(4, 4, &buf, 80)

	d.workers[0].start("https://ex.aa/file")
	d.workers[0].update(1048576, 2097152)
	d.workers[1].start("https://ex.bb/file")
	d.workers[1].update(524288, 1048576)
	d.workers[2].start("https://ex.cc/file")
	d.workers[2].update(0, 1048576)
	d.workers[3].start("https://ex.dd/file")
	d.workers[3].update(2097152, 4194304)

	d.render()
	got := buf.String()
	t.Logf("render output:\n%q", got)

	if !strings.HasPrefix(got, "\033[K") {
		t.Errorf("first render should start with \\033[K, got: %q", got[:20])
	}

	parts := strings.Split(got, "\n")
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts (2 rows + global via \\n), got %d", len(parts))
	}

	if !strings.Contains(parts[0], "ex.aa") {
		t.Errorf("row 0 should contain worker 0 URL")
	}
	if !strings.Contains(parts[0], "ex.bb") {
		t.Errorf("row 0 should contain worker 1 URL")
	}
	if !strings.Contains(parts[1], "ex.cc") {
		t.Errorf("row 1 should contain worker 2 URL")
	}
	if !strings.Contains(parts[1], "ex.dd") {
		t.Errorf("row 1 should contain worker 3 URL")
	}
	if !strings.Contains(parts[2], "[0/4]") {
		t.Errorf("global should contain [0/4], got: %q", parts[2])
	}

	n := strings.Count(got, "\033[K")
	if n != 3 {
		t.Errorf("expected 3 \\033[K (row0, row1, global), got %d", n)
	}
}

func TestRenderWithCompletedWorkers(t *testing.T) {
	var buf bytes.Buffer
	d := newTestDisplay(3, 5, &buf, 80)

	d.workers[0].start("https://ex.aa/file")
	d.workers[0].update(1048576, 2097152)
	d.workers[1].start("https://ex.bb/file")
	d.workers[1].done()
	d.workers[2].start("https://ex.cc/file")
	d.workers[2].update(524288, 1048576)

	d.render()
	got := buf.String()
	parts := strings.Split(got, "\n")

	if strings.Contains(parts[0], "ex.bb") {
		t.Errorf("row 0 should NOT contain done worker 1: %q", parts[0])
	}
	if !strings.Contains(parts[0], "ex.aa") {
		t.Errorf("row 0 should contain worker 0: %q", parts[0])
	}
	if !strings.Contains(parts[1], "ex.cc") {
		t.Errorf("row 1 should contain worker 2: %q", parts[1])
	}
	if strings.Contains(parts[2], "ex.") {
		t.Errorf("global line should not contain URL: %q", parts[2])
	}
}

func TestRenderFinalClearsAndPrintsDone(t *testing.T) {
	var buf bytes.Buffer
	d := newTestDisplay(4, 4, &buf, 80)

	d.workers[0].start("https://ex.aa/file")
	d.workers[0].update(0, 100)
	d.workers[1].start("https://ex.bb/file")
	d.workers[1].update(0, 100)
	d.workers[2].start("https://ex.cc/file")
	d.workers[2].update(0, 100)
	d.workers[3].start("https://ex.dd/file")
	d.workers[3].update(0, 100)

	d.render()
	buf.Reset()

	d.completed.Store(4)
	d.renderFinal()
	got := buf.String()
	t.Logf("renderFinal output:\n%q", got)

	if !strings.Contains(got, "[4/4] done") {
		t.Errorf("renderFinal should contain '[4/4] done', got: %q", got)
	}

	clearCount := strings.Count(got, "\033[K")
	if clearCount != 3 {
		t.Errorf("expected 3 \\033[K clear sequences, got %d", clearCount)
	}
	upCount := strings.Count(got, "\033[A")
	if upCount != 2 {
		t.Errorf("expected 2 \\033[A up sequences (rows=2), got %d", upCount)
	}
}

func TestRenderTwoWorkersOneRow(t *testing.T) {
	var buf bytes.Buffer
	d := newTestDisplay(2, 2, &buf, 80)

	d.workers[0].start("https://ex.aa/file")
	d.workers[0].update(1048576, 2097152)
	d.workers[1].start("https://ex.bb/file")
	d.workers[1].update(524288, 1048576)

	d.render()
	got := buf.String()
	parts := strings.Split(got, "\n")

	if len(parts) != 2 {
		t.Fatalf("expected 2 parts (1 row + global), got %d", len(parts))
	}
	if !strings.Contains(parts[0], "ex.aa") {
		t.Errorf("row 0 should contain worker 0")
	}
	if !strings.Contains(parts[0], "ex.bb") {
		t.Errorf("row 0 should contain worker 1")
	}
}

func TestRenderSecondFrameMovesUp(t *testing.T) {
	var buf bytes.Buffer
	d := newTestDisplay(4, 4, &buf, 80)

	d.workers[0].start("https://ex.aa/file")
	d.workers[0].update(0, 100)
	d.workers[1].start("https://ex.bb/file")
	d.workers[1].update(0, 100)
	d.workers[2].start("https://ex.cc/file")
	d.workers[2].update(0, 100)
	d.workers[3].start("https://ex.dd/file")
	d.workers[3].update(0, 100)

	d.render()
	buf.Reset()
	d.render()
	got := buf.String()

	if !strings.HasPrefix(got, "\033[2A") {
		t.Errorf("second render should start with \\033[2A, got: %q", got[:20])
	}
}

func TestRenderManyWorkers(t *testing.T) {
	var buf bytes.Buffer
	d := newTestDisplay(16, 20, &buf, 120)

	if d.rows != 8 {
		t.Fatalf("expected 8 rows for 16 workers, got %d", d.rows)
	}
	if len(d.workers) != 16 {
		t.Fatalf("expected 16 worker slots, got %d", len(d.workers))
	}

	for i := 0; i < 15; i++ {
		d.workers[i].start(fmt.Sprintf("https://ex.%02d/file", i))
		d.workers[i].update(0, 1048576)
	}

	d.render()
	got := buf.String()
	parts := strings.Split(got, "\n")

	if len(parts) != 9 {
		t.Fatalf("expected 9 parts (8 rows + global), got %d", len(parts))
	}
}

func TestRenderNoOutputForNonTTY(t *testing.T) {
	// Simulate tty=false
	var buf bytes.Buffer
	d := newTestDisplay(4, 4, &buf, 80)
	d.tty = false

	d.workers[0].start("https://ex.aa/file")
	d.render()
	if buf.Len() != 0 {
		t.Errorf("expected no output for non-TTY, got %q", buf.String())
	}
}
