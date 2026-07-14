//go:build ignore

package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/hinshun/vt10x"
)

var (
	cols         = envInt("PTY_LOGGER_COLS", 120)
	rows         = envInt("PTY_LOGGER_ROWS", 40)
	debounceSecs = 1.0
	periodicDump = 15 * time.Second
	pollInterval = 50 * time.Millisecond
	staleCheck   = 5 * time.Minute
	maxEmitted   = 10000
)

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n := 0
	for _, c := range v {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	if n == 0 {
		return def
	}
	return n
}

func isStatusBar(line string, row, totalRows int) bool {
	stripped := strings.TrimRight(line, " \t")
	content := strings.TrimSpace(stripped)
	if content == "" {
		return false
	}
	contentLen := utf8.RuneCountInString(content)
	totalLen := utf8.RuneCountInString(stripped)
	if row >= totalRows-3 && totalLen > contentLen*5 {
		return true
	}
	return false
}

func isSeparator(line string) bool {
	stripped := strings.TrimSpace(line)
	if stripped == "" {
		return false
	}
	for _, r := range stripped {
		switch r {
		case '─', '━', '═', '┈', '┄', '╌', '╍':
			continue
		default:
			return false
		}
	}
	return true
}

type PtyLogger struct {
	term         vt10x.Terminal
	lastChange   time.Time
	pending      bool
	cycleEmitted map[string]bool
}

func NewPtyLogger() *PtyLogger {
	term := vt10x.New(vt10x.WithSize(cols, rows))
	return &PtyLogger{
		term:         term,
		cycleEmitted: make(map[string]bool),
	}
}

func (p *PtyLogger) Feed(data []byte) {
	if bytes.Contains(data, []byte("\x1b[2J")) {
		p.cycleEmitted = make(map[string]bool)
	}

	p.term.Write(data)
	p.lastChange = time.Now()
	p.pending = true
}

func (p *PtyLogger) getContentLines() []string {
	raw := p.term.String()
	allLines := strings.Split(raw, "\n")
	var result []string
	for row, line := range allLines {
		if isStatusBar(line, row, rows) {
			continue
		}
		stripped := strings.TrimSpace(line)
		if stripped == "" {
			continue
		}
		if isSeparator(stripped) {
			continue
		}
		result = append(result, stripped)
	}
	return result
}

func (p *PtyLogger) CheckEmit() bool {
	if !p.pending {
		return false
	}
	if time.Since(p.lastChange).Seconds() < debounceSecs {
		return false
	}
	p.pending = false
	return p.emitDiff()
}

func (p *PtyLogger) ForceEmit() bool {
	p.pending = false
	return p.emitDiff()
}

func (p *PtyLogger) emitDiff() bool {
	lines := p.getContentLines()
	any := false
	for _, line := range lines {
		if p.cycleEmitted[line] {
			continue
		}
		fmt.Println(line)
		p.cycleEmitted[line] = true
		any = true
	}
	if len(p.cycleEmitted) > maxEmitted {
		p.cycleEmitted = make(map[string]bool)
	}
	return any
}

func tailFile(filepath string, done <-chan struct{}) <-chan []byte {
	ch := make(chan []byte, 100)
	go func() {
		defer close(ch)
		for {
			if _, err := os.Stat(filepath); err == nil {
				break
			}
			select {
			case <-done:
				return
			case <-time.After(100 * time.Millisecond):
			}
		}
		f, err := os.Open(filepath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[pty-logger] open %s: %v\n", filepath, err)
			return
		}
		defer f.Close()
		buf := make([]byte, 8192)
		lastData := time.Now()
		for {
			select {
			case <-done:
				return
			default:
			}
			n, err := f.Read(buf)
			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])
				ch <- data
				lastData = time.Now()
			}
			if err == io.EOF {
				if time.Since(lastData) > staleCheck {
					if os.Getppid() == 1 {
						fmt.Fprintln(os.Stderr, "[pty-logger] parent exited (adopted by init), shutting down")
						return
					}
				}
				time.Sleep(pollInterval)
				continue
			}
			if err != nil {
				return
			}
		}
	}()
	return ch
}

func main() {
	logger := NewPtyLogger()
	fmt.Fprintf(os.Stderr, "[pty-logger] cols=%d rows=%d debounce=%.1fs\n", cols, rows, debounceSecs)

	done := make(chan struct{})
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigCh
		close(done)
	}()

	var source <-chan []byte
	if len(os.Args) > 1 {
		fp := os.Args[1]
		fmt.Fprintf(os.Stderr, "[pty-logger] tailing %s\n", fp)
		source = tailFile(fp, done)
	} else {
		ch := make(chan []byte, 100)
		go func() {
			defer close(ch)
			buf := make([]byte, 8192)
			for {
				n, err := os.Stdin.Read(buf)
				if n > 0 {
					d := make([]byte, n)
					copy(d, buf[:n])
					ch <- d
				}
				if err != nil {
					return
				}
			}
		}()
		source = ch
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	pTicker := time.NewTicker(periodicDump)
	defer pTicker.Stop()

	for {
		select {
		case data, ok := <-source:
			if !ok {
				logger.ForceEmit()
				return
			}
			logger.Feed(data)
		case <-ticker.C:
			logger.CheckEmit()
		case <-pTicker.C:
			logger.ForceEmit()
		case <-done:
			logger.ForceEmit()
			return
		}
	}
}
