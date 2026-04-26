package character

import (
	"bufio"
	"context"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"jyzs_proxy_poc/internal/storage"
)

type Store interface {
	InsertCharacterStatus(storage.CharacterStatus) error
}

type Config struct {
	TraceLogPath  string
	SystemSetPath string
	PollInterval  time.Duration
}

type Collector struct {
	cfg         Config
	store       Store
	traceOffset int64
}

type traceLoginEvent struct {
	At             time.Time
	PlayerName     string
	LoginSucceeded bool
}

var traceLoginPattern = regexp.MustCompile(`SetLoginSucceed\s+(true|false)\s+PlayerName\[(.*?)\]`)

func NewCollector(cfg Config, store Store) *Collector {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 2 * time.Second
	}
	return &Collector{
		cfg:   cfg,
		store: store,
	}
}

func (c *Collector) Run(ctx context.Context) {
	_ = c.collectOnce()

	ticker := time.NewTicker(c.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = c.collectOnce()
		}
	}
}

func (c *Collector) collectOnce() error {
	if c.cfg.TraceLogPath == "" {
		return nil
	}

	meta, _ := loadSystemSet(c.cfg.SystemSetPath)

	f, err := os.Open(c.cfg.TraceLogPath)
	if err != nil {
		return err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return err
	}
	if stat.Size() < c.traceOffset {
		c.traceOffset = 0
	}

	if _, err := f.Seek(c.traceOffset, io.SeekStart); err != nil {
		return err
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		evt, ok := parseTraceLoginEvent(line)
		if !ok {
			continue
		}

		s := storage.CharacterStatus{
			At:             evt.At,
			PlayerName:     firstNonEmpty(evt.PlayerName, meta.SwitchPlayerName),
			LoginSucceeded: evt.LoginSucceeded,
			LoginAccount:   meta.LoginAccount,
			AreaName:       meta.CurAreaName,
			ServerName:     meta.CurServerName,
			Source:         "trace.log",
			RawLine:        line,
		}
		if s.At.IsZero() {
			s.At = time.Now()
		}
		if s.PlayerName == "" {
			continue
		}
		_ = c.store.InsertCharacterStatus(s)
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	pos, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}
	c.traceOffset = pos
	return nil
}

func parseTraceLoginEvent(line string) (traceLoginEvent, bool) {
	m := traceLoginPattern.FindStringSubmatch(line)
	if len(m) != 3 {
		return traceLoginEvent{}, false
	}
	event := traceLoginEvent{
		LoginSucceeded: strings.EqualFold(m[1], "true"),
		PlayerName:     strings.TrimSpace(m[2]),
	}
	if len(line) > 24 && line[0] == '[' {
		if idx := strings.IndexByte(line, ']'); idx > 1 {
			if parsed, err := time.Parse("2006-01-02 15:04:05.000", line[1:idx]); err == nil {
				event.At = parsed
			}
		}
	}
	return event, true
}

type systemSetMeta struct {
	LoginAccount     string
	CurAreaName      string
	CurServerName    string
	SwitchPlayerName string
}

func loadSystemSet(path string) (systemSetMeta, error) {
	if path == "" {
		return systemSetMeta{}, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return systemSetMeta{}, err
	}

	var out systemSetMeta
	scanner := bufio.NewScanner(strings.NewReader(string(b)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "[") || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key := strings.TrimSpace(k)
		val := strings.TrimSpace(v)
		switch key {
		case "login_account":
			out.LoginAccount = val
		case "cur_area_name":
			out.CurAreaName = val
		case "cur_server_name":
			out.CurServerName = val
		case "switch_player_name":
			out.SwitchPlayerName = val
		}
	}
	if err := scanner.Err(); err != nil {
		return systemSetMeta{}, err
	}
	return out, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
