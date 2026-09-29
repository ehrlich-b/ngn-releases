// Package history persists rolling measurement runs as append-only JSONL and
// weights them by commit distance from HEAD, so stale data decays as the repo
// advances. Shared by cmd/smoke and cmd/gauntlet.
package history

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Append writes v as one JSON line to path, creating the parent dir if needed.
func Append(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = f.Write(data)
	return err
}

// Load reads all records of type T from the JSONL file. A missing file returns
// (nil, nil) — callers treat that as "no history yet". Unparseable lines skip.
func Load[T any](path string) ([]T, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var out []T
	scan := bufio.NewScanner(f)
	scan.Buffer(make([]byte, 0, 4096), 1024*1024)
	for scan.Scan() {
		var r T
		if err := json.Unmarshal(scan.Bytes(), &r); err != nil {
			continue
		}
		out = append(out, r)
	}
	return out, scan.Err()
}

// CommitDistance counts commits in HEAD's history that aren't in sha's ancestry:
// 0 when sha == HEAD, positive as HEAD advances, and -1 if sha can't be resolved.
func CommitDistance(sha string) int {
	if sha == "" || sha == "unknown" {
		return -1
	}
	out, err := exec.Command("git", "rev-list", "--count", sha+"..HEAD").Output()
	if err != nil {
		return -1
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return -1
	}
	return n
}

// RecordWeight applies linear decay so older runs count for less:
//   - distance 0-5:   1.0 - 0.10*distance       (1.00 -> 0.50)
//   - distance 6-14:  0.50 - 0.05*(distance-5)  (0.45 -> 0.05)
//   - distance >= 15 or unreachable: 0
func RecordWeight(distance int) float64 {
	if distance < 0 || distance >= 15 {
		return 0
	}
	if distance <= 5 {
		return 1.0 - 0.10*float64(distance)
	}
	return 0.50 - 0.05*float64(distance-5)
}

// CurrentSHA returns HEAD's full SHA, or "unknown".
func CurrentSHA() string {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

// ShortSHA truncates a SHA to 7 chars.
func ShortSHA(sha string) string {
	if len(sha) >= 7 {
		return sha[:7]
	}
	return sha
}
