package k4label

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Analyzer interface {
	Provenance() TeacherProvenance
	Analyze(context.Context, string) (SearchResult, string, error)
	Close() error
}

type streamItem struct {
	line string
	err  error
}

type boundedWriter struct {
	mu        sync.Mutex
	data      []byte
	remaining int
}

func newBoundedWriter(limit int) *boundedWriter {
	return &boundedWriter{remaining: limit}
}

func (writer *boundedWriter) Write(data []byte) (int, error) {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	original := len(data)
	if len(data) > writer.remaining {
		data = data[:writer.remaining]
	}
	writer.data = append(writer.data, data...)
	writer.remaining -= len(data)
	return original, nil
}

func (writer *boundedWriter) String() string {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	return string(writer.data)
}

type Session struct {
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	lines      chan streamItem
	done       chan struct{}
	waitMu     sync.Mutex
	waitErr    error
	stderr     *boundedWriter
	config     SearchConfig
	provenance TeacherProvenance
	closeOnce  sync.Once
	closeErr   error
}

func sha256Path(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func validateProvenance(sourceCommit, bigNetworkSHA, smallNetworkSHA string) error {
	if sourceCommit != TeacherSourceCommit {
		return fmt.Errorf("%w: teacher source commit %q, want pinned %s", ErrContract, sourceCommit, TeacherSourceCommit)
	}
	if bigNetworkSHA != TeacherBigNetworkSHA || smallNetworkSHA != TeacherSmallNetSHA {
		return fmt.Errorf("%w: teacher network hashes do not match frozen Stockfish 18 BIG/SMALL networks", ErrContract)
	}
	return nil
}

// StartSession launches one single-thread teacher and freezes its executable,
// source and network identities into the returned provenance. command[0] is the
// executable; additional elements are supported for protocol fixtures only.
func StartSession(ctx context.Context, command []string, sourceCommit, bigNetworkSHA, smallNetworkSHA string) (*Session, error) {
	return startSession(ctx, command, sourceCommit, bigNetworkSHA, smallNetworkSHA, FrozenSearchConfig())
}

// StartDiagnosticSession uses the frozen teacher settings with a larger node
// budget solely for the predeclared 5k-versus-20k calibration audit. It cannot
// be passed to RunShard: that function always requires FrozenSearchConfig.
func StartDiagnosticSession(ctx context.Context, command []string, sourceCommit, bigNetworkSHA, smallNetworkSHA string) (*Session, error) {
	config := FrozenSearchConfig()
	config.Nodes = 20_000
	config.TimeoutMillis = 8_000
	return startSession(ctx, command, sourceCommit, bigNetworkSHA, smallNetworkSHA, config)
}

func startSession(ctx context.Context, command []string, sourceCommit, bigNetworkSHA, smallNetworkSHA string, config SearchConfig) (*Session, error) {
	if len(command) == 0 || command[0] == "" {
		return nil, fmt.Errorf("%w: empty teacher command", ErrContract)
	}
	if err := validateProvenance(sourceCommit, bigNetworkSHA, smallNetworkSHA); err != nil {
		return nil, err
	}
	executableSHA, err := sha256Path(command[0])
	if err != nil {
		return nil, fmt.Errorf("hash teacher executable: %w", err)
	}
	cmd := exec.Command(command[0], command[1:]...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr := newBoundedWriter(1 << 20)
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	session := &Session{
		cmd: cmd, stdin: stdin, lines: make(chan streamItem, 4096), done: make(chan struct{}),
		stderr: stderr, config: config,
	}
	go func() {
		err := cmd.Wait()
		session.waitMu.Lock()
		session.waitErr = err
		session.waitMu.Unlock()
		close(session.done)
	}()
	go func() {
		defer close(session.lines)
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 64*1024), 4<<20)
		for scanner.Scan() {
			select {
			case session.lines <- streamItem{line: strings.TrimSpace(scanner.Text())}:
			case <-session.done:
				return
			}
		}
		if err := scanner.Err(); err != nil {
			select {
			case session.lines <- streamItem{err: err}:
			case <-session.done:
			}
		}
	}()
	fail := func(cause error) (*Session, error) {
		_ = session.abort()
		return nil, cause
	}
	if err := session.send("uci"); err != nil {
		return fail(err)
	}
	var name string
	var options []string
	for {
		line, err := session.next(ctx)
		if err != nil {
			return fail(fmt.Errorf("teacher UCI handshake: %w", err))
		}
		switch {
		case strings.HasPrefix(line, "id name "):
			name = strings.TrimSpace(strings.TrimPrefix(line, "id name "))
		case strings.HasPrefix(line, "option name "):
			options = append(options, line)
		case line == "uciok":
			goto configured
		}
	}

configured:
	if name == "" {
		return fail(fmt.Errorf("%w: teacher omitted UCI id name", ErrContract))
	}
	if !strings.HasPrefix(name, "Stockfish 18") {
		return fail(fmt.Errorf("%w: teacher UCI name %q is not Stockfish 18", ErrContract, name))
	}
	for _, required := range []string{"Threads", "Hash", "MultiPV", "SyzygyPath"} {
		found := false
		prefix := "option name " + required + " "
		for _, option := range options {
			if strings.HasPrefix(option, prefix) {
				found = true
				break
			}
		}
		if !found {
			return fail(fmt.Errorf("%w: teacher omitted required UCI option %s", ErrContract, required))
		}
	}
	for _, option := range []string{
		"setoption name Threads value 1",
		"setoption name Hash value 16",
		"setoption name MultiPV value 1",
		"setoption name SyzygyPath value <empty>",
	} {
		if err := session.send(option); err != nil {
			return fail(err)
		}
	}
	if err := session.send("isready"); err != nil {
		return fail(err)
	}
	if err := session.waitExact(ctx, "readyok"); err != nil {
		return fail(fmt.Errorf("teacher option readiness: %w", err))
	}
	optionDigest := sha256.Sum256([]byte(strings.Join(options, "\n") + "\n"))
	session.provenance = TeacherProvenance{
		Name: name, ExecutablePath: command[0], ExecutableSHA256: executableSHA,
		SourceCommit: sourceCommit, BigNetworkSHA256: bigNetworkSHA,
		SmallNetworkSHA256: smallNetworkSHA, Command: append([]string(nil), command...),
		HandshakeOptionDigest: hex.EncodeToString(optionDigest[:]),
	}
	return session, nil
}

func (session *Session) Provenance() TeacherProvenance { return session.provenance }

func (session *Session) send(command string) error {
	if strings.ContainsAny(command, "\r\n") {
		return fmt.Errorf("%w: multiline UCI command", ErrContract)
	}
	_, err := io.WriteString(session.stdin, command+"\n")
	return err
}

func (session *Session) next(ctx context.Context) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case item, ok := <-session.lines:
		if !ok {
			return "", fmt.Errorf("teacher stdout closed; stderr=%q", session.stderr.String())
		}
		if item.err != nil {
			return "", fmt.Errorf("teacher stdout: %w", item.err)
		}
		return item.line, nil
	}
}

func (session *Session) waitExact(ctx context.Context, expected string) error {
	for {
		line, err := session.next(ctx)
		if err != nil {
			return err
		}
		if line == expected {
			return nil
		}
		if protocolErrorLine(line) {
			return fmt.Errorf("teacher protocol error %q", line)
		}
	}
}

func protocolErrorLine(line string) bool {
	lower := strings.ToLower(line)
	return strings.HasPrefix(lower, "unknown command") || strings.HasPrefix(lower, "error:") ||
		strings.HasPrefix(lower, "info string error:")
}

type infoScore struct {
	hasScore bool
	cp       int
	mate     bool
	bound    bool
	depth    int
	seldepth int
	nodes    uint64
	pv       string
}

func parseInfo(line string) (infoScore, error) {
	var result infoScore
	fields := strings.Fields(line)
	if len(fields) == 0 || fields[0] != "info" {
		return result, nil
	}
	multipv := 1
	for index := 1; index < len(fields); index++ {
		parseInt := func(name string) (int, error) {
			if index+1 >= len(fields) {
				return 0, fmt.Errorf("%s missing value", name)
			}
			value, err := strconv.Atoi(fields[index+1])
			if err != nil {
				return 0, fmt.Errorf("%s value %q: %w", name, fields[index+1], err)
			}
			index++
			return value, nil
		}
		switch fields[index] {
		case "depth":
			value, err := parseInt("depth")
			if err != nil {
				return result, err
			}
			result.depth = value
		case "seldepth":
			value, err := parseInt("seldepth")
			if err != nil {
				return result, err
			}
			result.seldepth = value
		case "nodes":
			if index+1 >= len(fields) {
				return result, errors.New("nodes missing value")
			}
			value, err := strconv.ParseUint(fields[index+1], 10, 64)
			if err != nil {
				return result, fmt.Errorf("nodes value %q: %w", fields[index+1], err)
			}
			result.nodes = value
			index++
		case "multipv":
			value, err := parseInt("multipv")
			if err != nil {
				return result, err
			}
			multipv = value
		case "score":
			if index+2 >= len(fields) {
				return result, errors.New("score missing kind/value")
			}
			kind := fields[index+1]
			value, err := strconv.Atoi(fields[index+2])
			if err != nil || value < -1_000_000_000 || value > 1_000_000_000 {
				return result, fmt.Errorf("invalid score %q", fields[index+2])
			}
			result.hasScore = true
			result.cp = value
			result.mate = kind == "mate"
			if kind != "cp" && kind != "mate" {
				return result, fmt.Errorf("unsupported score kind %q", kind)
			}
			index += 2
		case "lowerbound", "upperbound":
			result.bound = true
		case "pv":
			if index+1 < len(fields) {
				result.pv = fields[index+1]
			}
			index = len(fields)
		}
	}
	if multipv != 1 {
		return infoScore{}, nil
	}
	return result, nil
}

func parseBestMove(line string) (string, error) {
	fields := strings.Fields(line)
	if len(fields) < 2 || fields[0] != "bestmove" || fields[1] == "" {
		return "", fmt.Errorf("malformed bestmove line %q", line)
	}
	return fields[1], nil
}

func (session *Session) Analyze(parent context.Context, fen string) (SearchResult, string, error) {
	ctx, cancel := context.WithTimeout(parent, time.Duration(session.config.TimeoutMillis)*time.Millisecond)
	defer cancel()
	for _, command := range []string{"ucinewgame", "isready"} {
		if err := session.send(command); err != nil {
			_ = session.abort()
			return SearchResult{}, "", err
		}
	}
	if err := session.waitExact(ctx, "readyok"); err != nil {
		_ = session.abort()
		return SearchResult{}, "", fmt.Errorf("teacher root reset: %w", err)
	}
	if err := session.send("position fen " + fen); err != nil {
		_ = session.abort()
		return SearchResult{}, "", err
	}
	if err := session.send(fmt.Sprintf("go nodes %d", session.config.Nodes)); err != nil {
		_ = session.abort()
		return SearchResult{}, "", err
	}
	var latest *SearchResult
	var observed SearchResult
	sawMate := false
	sawBound := false
	sawCP := false
	for {
		line, err := session.next(ctx)
		if err != nil {
			_ = session.abort()
			return SearchResult{}, "", fmt.Errorf("teacher search: %w", err)
		}
		if protocolErrorLine(line) {
			_ = session.abort()
			return SearchResult{}, "", fmt.Errorf("teacher protocol error %q", line)
		}
		if strings.HasPrefix(line, "info ") {
			parsed, err := parseInfo(line)
			if err != nil {
				_ = session.abort()
				return SearchResult{}, "", fmt.Errorf("parse teacher info %q: %w", line, err)
			}
			if parsed.hasScore {
				observed = SearchResult{CP: parsed.cp, Depth: parsed.depth, SelDepth: parsed.seldepth, Nodes: parsed.nodes, PVMove: parsed.pv, InfoLine: line}
				sawMate = sawMate || parsed.mate
				sawBound = sawBound || parsed.bound
				sawCP = sawCP || !parsed.mate
				if parsed.mate {
					latest = nil
				} else if !parsed.bound {
					copy := observed
					latest = &copy
				}
			}
			continue
		}
		if !strings.HasPrefix(line, "bestmove ") {
			continue
		}
		best, err := parseBestMove(line)
		if err != nil {
			return SearchResult{}, "", err
		}
		if latest == nil {
			reason := "missing-score"
			if sawMate {
				reason = "mate-score"
			} else if sawBound || sawCP {
				reason = "bound-only-score"
			}
			observed.BestMove = best
			return observed, reason, nil
		}
		latest.BestMove = best
		switch {
		case latest.Depth < session.config.MinimumDepth:
			return *latest, "depth-below-minimum", nil
		case latest.PVMove == "":
			return *latest, "missing-pv", nil
		default:
			return *latest, "", nil
		}
	}
}

func (session *Session) abort() error {
	if session.cmd.Process != nil {
		_ = session.cmd.Process.Kill()
	}
	select {
	case <-session.done:
		session.waitMu.Lock()
		defer session.waitMu.Unlock()
		return session.waitErr
	case <-time.After(2 * time.Second):
		return errors.New("teacher process did not exit after kill")
	}
}

func (session *Session) Close() error {
	session.closeOnce.Do(func() {
		_ = session.send("quit")
		select {
		case <-session.done:
			session.waitMu.Lock()
			session.closeErr = session.waitErr
			session.waitMu.Unlock()
		case <-time.After(2 * time.Second):
			session.closeErr = session.abort()
		}
	})
	return session.closeErr
}
