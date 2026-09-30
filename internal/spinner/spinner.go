package spinner

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

var frames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner renders an animated progress indicator on stderr.
// It is a no-op when stderr is not a terminal.
type Spinner struct {
	out  io.Writer
	mu   sync.Mutex
	msg  string
	stop chan struct{}
	done chan struct{}
	once sync.Once
}

// Start begins spinning with the given message.
func Start(msg string) *Spinner {
	s := &Spinner{msg: msg}
	if !isTerminal(os.Stderr) {
		return s
	}
	s.out = os.Stderr
	s.stop = make(chan struct{})
	s.done = make(chan struct{})
	go s.run()
	return s
}

// Update changes the message shown next to the spinner.
func (s *Spinner) Update(msg string) {
	s.mu.Lock()
	s.msg = msg
	s.mu.Unlock()
}

// Stop halts the spinner and clears its line. Safe to call multiple times.
func (s *Spinner) Stop() {
	if s.out == nil {
		return
	}
	s.once.Do(func() {
		close(s.stop)
		<-s.done
	})
}

func (s *Spinner) run() {
	defer close(s.done)
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()

	for i := 0; ; i++ {
		s.mu.Lock()
		msg := s.msg
		s.mu.Unlock()
		_, _ = fmt.Fprintf(s.out, "\r\033[K%s %s", frames[i%len(frames)], msg)

		select {
		case <-s.stop:
			_, _ = fmt.Fprint(s.out, "\r\033[K")
			return
		case <-ticker.C:
		}
	}
}

func isTerminal(f *os.File) bool {
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return stat.Mode()&os.ModeCharDevice != 0
}
