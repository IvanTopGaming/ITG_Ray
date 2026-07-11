package geo

import (
	"context"
	"fmt"
	"io"
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
	for _, tag := range tags {
		if !force {
			if p, ok := m.cached(preset, tag); ok {
				out[tag] = p
				done++
				m.report(done, total)
				continue
			}
		}
		data, err := m.download(ctx, directURL(tag, base))
		if err != nil {
			return nil, fmt.Errorf("geo: fetch %q from %s: %w", tag, preset, err)
		}
		p, err := m.writeCache(preset, tag, data)
		if err != nil {
			return nil, fmt.Errorf("geo: cache %q: %w", tag, err)
		}
		out[tag] = p
		done++
		m.report(done, total)
	}
	return out, nil
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
