package main

import (
	"fmt"
	"os"
	"sync"
)

type Progress struct {
	mu    sync.Mutex
	tty   bool
	index int
	total int
}

func NewProgress() *Progress {
	fi, err := os.Stderr.Stat()
	tty := err == nil && (fi.Mode()&os.ModeCharDevice) != 0
	return &Progress{tty: tty}
}

func (p *Progress) SetTask(index, total int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.index = index
	p.total = total
}

func (p *Progress) Update(url string) {
	if !p.tty {
		return
	}
	p.mu.Lock()
	index, total := p.index, p.total
	p.mu.Unlock()
	fmt.Fprintf(os.Stderr, "\r\033[K[%d/%d] fetching '%s'", index+1, total, url)
}

func (p *Progress) Done() {
	if !p.tty {
		return
	}
	p.mu.Lock()
	total := p.total
	p.mu.Unlock()
	if total > 0 {
		fmt.Fprintf(os.Stderr, "\r\033[K[%d/%d] done\n", total, total)
	} else {
		fmt.Fprint(os.Stderr, "\r\033[K")
	}
}
