package runtime

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"porter/internal/store"
	"porter/internal/types"
)

// Collector tails per-VM Firecracker log files into the store rings and records
// a heartbeat metric per VM (manual §8: logger once, metrics 60s + Flush).
// It is best-effort: missing files (dev/Windows) simply yield nothing.
type Collector struct {
	store   *store.Store
	logsDir string
}

// NewCollector builds the log/metrics consumer for one logs dir.
func NewCollector(st *store.Store, logsDir string) *Collector {
	return &Collector{store: st, logsDir: logsDir}
}

// PollOnce tails <logsDir>/<vmID>.log (if present) into store logs and records
// one liveness metric per supplied VM id. Values are counts/strings only.
func (c *Collector) PollOnce(vmIDs []string) {
	if c == nil || c.store == nil {
		return
	}
	for _, id := range vmIDs {
		path := filepath.Join(c.logsDir, id+".log")
		if lines := tailFile(path, 20); len(lines) > 0 {
			for _, ln := range lines {
				ln = strings.TrimSpace(ln)
				if ln == "" || strings.Contains(ln, "API_KEY") || strings.Contains(ln, "SECRET") {
					continue
				}
				c.store.AppendLog(id, ln)
			}
		}
		_ = c.store.AddMetric(&types.MetricSample{
			VMID: id, Metric: "porter.heartbeat",
			Value: 1, TS: time.Now(),
		})
	}
}

// Start polls every interval until ctx ends.
func (c *Collector) Start(ctx context.Context, interval time.Duration, list func() []string) {
	if interval <= 0 {
		interval = 60 * time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c.PollOnce(list())
		}
	}
}

func tailFile(path string, n int) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		lines = append(lines, sc.Text())
		if len(lines) > 200 {
			lines = lines[len(lines)-200:]
		}
	}
	if len(lines) > n {
		return lines[len(lines)-n:]
	}
	return lines
}
