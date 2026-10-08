package claim

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type wireLine struct {
	index int
	text  string
}
type Session struct {
	cmd       *exec.Cmd
	in        io.WriteCloser
	lines     chan wireLine
	done      chan error
	abort     chan struct{}
	start     time.Time
	mu        sync.Mutex
	writeMu   sync.Mutex
	closeOnce sync.Once
	closeErr  error
	record    SessionRecord
	bad       error
}

var ProcessContext = context.Background()

func engineError(line string) bool {
	l := strings.ToLower(strings.TrimSpace(line))
	if strings.Contains(l, "panic") || strings.Contains(l, "fatal") || strings.Contains(l, "segmentation fault") || strings.Contains(l, "runtime error:") {
		return true
	}
	if strings.HasPrefix(l, "info string ") {
		for _, word := range []string{"error", "warning", "invalid", "failed", "failure", "unknown option", "unknown command", "illegal"} {
			if strings.Contains(l, word) {
				return true
			}
		}
	}
	if !strings.HasPrefix(l, "option ") && !strings.HasPrefix(l, "id ") {
		for _, word := range strings.Fields(l) {
			if strings.Trim(word, ":;,[]()") == "error" {
				return true
			}
		}
	}
	return strings.Contains(l, "unknown option") || strings.Contains(l, "unknown command") || strings.Contains(l, "invalid option") || strings.Contains(l, "illegal move") || strings.HasPrefix(l, "failed ") || strings.HasPrefix(l, "warning:") || strings.HasPrefix(l, "warning ") || strings.HasPrefix(l, "warn ")
}
func (s *Session) event(stream, line string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := len(s.record.Events)
	s.record.Events = append(s.record.Events, Event{time.Since(s.start).Nanoseconds(), stream, line})
	if stream != "stdin" && engineError(line) {
		s.bad = fmt.Errorf("engine error: %s", line)
	}
	return i
}

// Serialize command writes with their timestamps so stop/quit cannot interleave
// bytes with a position/go command. Search time includes pipe/collector overhead.
func (s *Session) send(line string) (int, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	i := s.event("stdin", line)
	_, e := io.WriteString(s.in, line+"\n")
	return i, e
}
func (s *Session) health() error { s.mu.Lock(); defer s.mu.Unlock(); return s.bad }
func (s *Session) until(ctx context.Context, token string) (wireLine, error) {
	for {
		select {
		case <-ctx.Done():
			return wireLine{}, fmt.Errorf("watchdog: waiting for %s: %w", token, ctx.Err())
		case l, ok := <-s.lines:
			if !ok {
				return l, fmt.Errorf("unexpected engine EOF waiting for %s", token)
			}
			if e := s.health(); e != nil {
				return l, e
			}
			if l.text == token || strings.HasPrefix(l.text, token+" ") {
				return l, nil
			}
		}
	}
}
func (s *Session) ready() error {
	if _, e := s.send("isready"); e != nil {
		return e
	}
	c, cancel := context.WithTimeout(ProcessContext, 15*time.Second)
	defer cancel()
	_, e := s.until(c, "readyok")
	return e
}
func StartSession(a Artifact, selected map[string]string) (*Session, error) {
	if e := Check(a); e != nil {
		return nil, e
	}
	if strings.HasSuffix(strings.ToLower(a.Path), ".exe") {
		if e := Check(WindowsBridge); e != nil {
			return nil, fmt.Errorf("Windows bridge: %w", e)
		}
	}
	s := &Session{cmd: sessionCommand(a), lines: make(chan wireLine, 4096), done: make(chan error, 1), abort: make(chan struct{}), start: time.Now(), record: SessionRecord{Engine: a, Options: map[string]string{}, Selected: map[string]string{}, ExitCode: -1, LaunchArgv: sessionLaunch(a), Scheduling: schedulingFor(a)}}
	s.cmd.Dir = filepath.Dir(a.Path)
	s.cmd.Env = append(os.Environ(), "GOMAXPROCS=1")
	var e error
	s.in, e = s.cmd.StdinPipe()
	if e != nil {
		return nil, e
	}
	out, e := s.cmd.StdoutPipe()
	if e != nil {
		return nil, e
	}
	errout, e := s.cmd.StderrPipe()
	if e != nil {
		return nil, e
	}
	if e = s.cmd.Start(); e != nil {
		return nil, e
	}
	s.record.PID = s.cmd.Process.Pid
	var readers sync.WaitGroup
	readers.Add(2)
	go func() {
		defer readers.Done()
		defer close(s.lines)
		sc := bufio.NewScanner(out)
		sc.Buffer(make([]byte, 65536), 4<<20)
		for sc.Scan() {
			l := sc.Text()
			i := s.event("stdout", l)
			select {
			case s.lines <- wireLine{i, l}:
			case <-s.abort:
			}
		}
		if e := sc.Err(); e != nil {
			s.mu.Lock()
			s.bad = e
			s.mu.Unlock()
		}
	}()
	go func() {
		defer readers.Done()
		sc := bufio.NewScanner(errout)
		sc.Buffer(make([]byte, 65536), 4<<20)
		for sc.Scan() {
			s.event("stderr", sc.Text())
		}
		if e := sc.Err(); e != nil {
			s.mu.Lock()
			s.bad = e
			s.mu.Unlock()
		}
	}()
	go func() { readers.Wait(); s.done <- s.cmd.Wait() }()
	fail := func(e error) (*Session, error) { s.Close(); return s, e }
	if _, e = s.send("uci"); e != nil {
		return fail(e)
	}
	c, cancel := context.WithTimeout(ProcessContext, 15*time.Second)
	defer cancel()
	for {
		select {
		case <-c.Done():
			return fail(fmt.Errorf("UCI handshake timeout"))
		case l, ok := <-s.lines:
			if !ok {
				return fail(fmt.Errorf("UCI handshake EOF"))
			}
			if e = s.health(); e != nil {
				return fail(e)
			}
			if strings.HasPrefix(l.text, "option name ") {
				parts := strings.SplitN(strings.TrimPrefix(l.text, "option name "), " type ", 2)
				if len(parts) != 2 || parts[0] == "" {
					return fail(fmt.Errorf("malformed UCI option"))
				}
				if _, exists := s.record.Options[parts[0]]; exists {
					return fail(fmt.Errorf("duplicate UCI option"))
				}
				s.record.Options[parts[0]] = parts[1]
			}
			if l.text == "uciok" {
				goto configured
			}
		}
	}
configured:
	if len(selected) == 0 {
		selected = FrozenOptions(s.record.Options, nil, 100)
	}
	// EvalFile must precede UseNNUE. Every other persistent option is frozen.
	keys := make([]string, 0, len(selected))
	for k := range selected {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	ordered := []string{}
	for _, k := range []string{"EvalFile", "UseNNUE"} {
		if _, ok := selected[k]; ok {
			ordered = append(ordered, k)
		}
	}
	for _, k := range keys {
		if k != "EvalFile" && k != "UseNNUE" {
			ordered = append(ordered, k)
		}
	}
	for _, k := range ordered {
		if _, ok := s.record.Options[k]; !ok {
			return fail(fmt.Errorf("unadvertised %s", k))
		}
		if _, e = s.send("setoption name " + k + " value " + selected[k]); e != nil {
			return fail(e)
		}
		s.record.Selected[k] = selected[k]
	}
	if e = s.ready(); e != nil {
		return fail(e)
	}
	return s, nil
}
func optionDefault(desc string) string {
	p := strings.SplitN(desc, " default ", 2)
	if len(p) != 2 {
		return ""
	}
	v := p[1]
	for _, delimiter := range []string{" min ", " max ", " var "} {
		v = strings.SplitN(v, delimiter, 2)[0]
	}
	return v
}
func FrozenOptions(options map[string]string, id *Identity, overhead int) map[string]string {
	r := map[string]string{}
	for k, d := range options {
		if v := optionDefault(d); v != "" {
			r[k] = v
		}
	}
	for k := range options {
		switch strings.ToLower(k) {
		case "threads":
			r[k] = "1"
		case "hash":
			r[k] = "64"
		case "ownbook", "usebook", "book", "ponder":
			r[k] = "false"
		case "move overhead":
			r[k] = fmt.Sprint(overhead)
		}
	}
	if id != nil {
		r["EvalFile"] = id.Net.Path
		r["UseNNUE"] = "true"
		if id.Net.Path == "" {
			r["EvalFile"] = "<empty>"
			r["UseNNUE"] = "false"
		}
	}
	return r
}
func (s *Session) Snapshot() SessionRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.record
	r.Events = append([]Event(nil), r.Events...)
	r.Options = map[string]string{}
	for k, v := range s.record.Options {
		r.Options[k] = v
	}
	r.Selected = map[string]string{}
	for k, v := range s.record.Selected {
		r.Selected[k] = v
	}
	return r
}
func (s *Session) Close() error {
	s.closeOnce.Do(func() {
		close(s.abort)
		_, sendErr := s.send("quit")
		s.in.Close()
		var e error
		select {
		case e = <-s.done:
		case <-time.After(2 * time.Second):
			s.cmd.Process.Kill()
			e = <-s.done
			if e == nil {
				e = fmt.Errorf("engine quit timeout")
			}
		}
		s.mu.Lock()
		if s.cmd.ProcessState != nil {
			s.record.ExitCode = s.cmd.ProcessState.ExitCode()
		}
		s.mu.Unlock()
		if e != nil {
			s.closeErr = e
		} else if sendErr != nil {
			s.closeErr = sendErr
		} else {
			s.closeErr = s.health()
		}
	})
	return s.closeErr
}
func (s *Session) Search(position, goCmd string, bank int64) (string, int, int, error) {
	if bank <= 0 {
		return "", 0, 0, fmt.Errorf("empty clock bank")
	}
	if _, e := s.send(position); e != nil {
		return "", 0, 0, e
	}
	g, e := s.send(goCmd)
	if e != nil {
		return "", g, 0, e
	}
	c, cancel := context.WithTimeout(ProcessContext, time.Duration(bank)+2*time.Second)
	defer cancel()
	b, e := s.until(c, "bestmove")
	if e != nil {
		return "", g, 0, e
	}
	f := strings.Fields(b.text)
	if len(f) != 2 && !(len(f) == 4 && f[2] == "ponder") {
		return "", g, b.index, fmt.Errorf("malformed bestmove: %s", b.text)
	}
	return f[1], g, b.index, nil
}
