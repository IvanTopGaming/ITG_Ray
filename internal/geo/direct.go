package geo

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

func directURL(tag, base string) string {
	base = strings.TrimRight(base, "/")
	if strings.HasPrefix(tag, "geoip-") {
		return base + "/sing-geoip/rule-set/" + tag + ".srs"
	}
	return base + "/sing-geosite/rule-set/" + tag + ".srs"
}

func (m *Manager) fetchDirect(ctx context.Context, preset, base string, tags []string, force bool) (map[string]string, error) {
	out := make(map[string]string, len(tags))
	total := int64(len(tags))
	var done int64
	var skipped []string
	for _, tag := range tags {
		if !force {
			if p, ok := m.cached(preset, tag); ok {
				out[tag] = p
				done++
				m.report(done, total)
				slog.Debug("geo: tag cached", slog.String("scope", "geo"), slog.String("tag", tag))
				continue
			}
		}
		data, err := m.downloadFirst(ctx, base, sourceNames(tag))
		if err != nil {
			skipped = append(skipped, tag)
			slog.Warn("geo: tag unavailable", slog.String("scope", "geo"), slog.String("tag", tag), slog.String("err", err.Error()))
			continue
		}
		p, err := m.writeCache(preset, tag, data)
		if err != nil {
			skipped = append(skipped, tag)
			slog.Warn("geo: tag cache write failed", slog.String("scope", "geo"), slog.String("tag", tag), slog.String("err", err.Error()))
			continue
		}
		out[tag] = p
		done++
		m.report(done, total)
		slog.Debug("geo: tag downloaded", slog.String("scope", "geo"), slog.String("tag", tag))
	}
	if len(out) == 0 && len(tags) > 0 {
		return nil, fmt.Errorf("geo: no tags available from %s (skipped %v)", preset, skipped)
	}
	return out, nil
}

func (m *Manager) downloadFirst(ctx context.Context, base string, names []string) ([]byte, error) {
	var lastErr error
	for _, n := range names {
		data, err := m.download(ctx, directURL(n, base))
		if err == nil {
			return data, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func (m *Manager) download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := m.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func (m *Manager) downloadProgress(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := m.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	total := resp.ContentLength
	var buf []byte
	tmp := make([]byte, 64*1024)
	var done int64
	for {
		n, rerr := resp.Body.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			done += int64(n)
			m.report(done, total)
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return nil, rerr
		}
	}
	return buf, nil
}
