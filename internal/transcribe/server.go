package transcribe

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"transflowy/locales"
)

const idleStop = 5 * time.Minute // the model holds about a gigabyte of memory

// Server runs whisper-server on a free local port while it is needed.
type Server struct {
	Model string // path to the ggml model

	mu   sync.Mutex
	cmd  *exec.Cmd
	port int
	idle *time.Timer
}

// FindBinary looks for whisper-server: bundled in the app, then on PATH, then
// where Homebrew puts it.
func FindBinary() (string, error) {
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), "..", "Resources", "bin", "whisper-server")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	if p, err := exec.LookPath("whisper-server"); err == nil {
		return p, nil
	}
	for _, p := range []string{"/opt/homebrew/bin/whisper-server", "/usr/local/bin/whisper-server"} {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", errors.New(locales.ErrNoBinary())
}

// URL starts the server when it is not running and returns its endpoint. A
// server that crashed is started again by the next call.
func (s *Server) URL(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.idle != nil {
		s.idle.Stop()
	}
	defer func() { s.idle = time.AfterFunc(idleStop, s.Stop) }()
	if s.cmd == nil {
		if err := s.start(ctx); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("http://127.0.0.1:%d/inference", s.port), nil
}

func (s *Server) start(ctx context.Context) error {
	bin, err := FindBinary()
	if err != nil {
		return err
	}
	if _, err := os.Stat(s.Model); err != nil {
		return errors.New(locales.ErrNoModel())
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	cmd := exec.Command(bin, "-m", s.Model, "--host", "127.0.0.1", "--port", fmt.Sprint(port), "-l", "auto")
	if err := cmd.Start(); err != nil {
		return err
	}
	exited := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		s.mu.Lock()
		if s.cmd == cmd {
			s.cmd = nil
		}
		s.mu.Unlock()
		close(exited)
	}()
	// The model takes seconds to load: wait until the port answers.
	deadline := time.After(90 * time.Second)
	for {
		if c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond); err == nil {
			c.Close()
			s.cmd, s.port = cmd, port
			return nil
		}
		select {
		case <-exited:
			return errors.New(locales.ErrServerExit())
		case <-deadline:
			_ = cmd.Process.Kill()
			return errors.New(locales.ErrServerTimeout())
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

// Stop ends the server. It is also called when the app quits.
func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.idle != nil {
		s.idle.Stop()
	}
	if s.cmd != nil {
		_ = s.cmd.Process.Kill()
		s.cmd = nil
	}
}
