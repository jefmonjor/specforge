package ui

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// Activity shows that something is running. On a terminal it animates a
// spinner with the elapsed time; elsewhere (CI logs) it prints one line, so
// logs are not flooded with carriage returns. The returned func stops it
// and waits until the line is cleared.
func Activity(w io.Writer, label string) func() {
	if !IsTerminal(w) {
		fmt.Fprintf(w, "  … %s\n", label)
		return func() {}
	}
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		start := time.Now()
		tick := time.NewTicker(120 * time.Millisecond)
		defer tick.Stop()
		for i := 0; ; i++ {
			select {
			case <-done:
				fmt.Fprint(w, "\r\033[K")
				return
			case <-tick.C:
				fmt.Fprintf(w, "\r\033[K  %s %s (%ds)", frames[i%len(frames)], label, int(time.Since(start).Seconds()))
			}
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() {
			close(done)
			wg.Wait()
		})
	}
}
