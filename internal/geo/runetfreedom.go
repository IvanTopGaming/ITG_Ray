package geo

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
)

const runetfreedomZipURL = "https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/sing-box.zip"

func runetfreedomZipEntry(tag string) string {
	if strings.HasPrefix(tag, "geoip-") {
		return "rule-set-geoip/" + tag + ".srs"
	}
	return "rule-set-geosite/" + tag + ".srs"
}

func (m *Manager) fetchRunetfreedom(ctx context.Context, tags []string, force bool) (map[string]string, error) {
	out := make(map[string]string, len(tags))
	var missing []string
	for _, tag := range tags {
		if !force {
			if p, ok := m.cached(PresetRunetfreedom, tag); ok {
				out[tag] = p
				continue
			}
		}
		missing = append(missing, tag)
	}
	if len(missing) == 0 {
		m.report(int64(len(tags)), int64(len(tags)))
		return out, nil
	}

	url := runetfreedomZipURL
	if m.zipURLOverride != "" {
		url = m.zipURLOverride
	}
	data, err := m.downloadProgress(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("geo: download runetfreedom zip: %w", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("geo: open runetfreedom zip: %w", err)
	}
	index := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		index[f.Name] = f
	}
	var skipped []string
	for _, tag := range missing {
		var f *zip.File
		for _, n := range sourceNames(tag) {
			if hit, ok := index[runetfreedomZipEntry(n)]; ok {
				f = hit
				break
			}
		}
		if f == nil {
			skipped = append(skipped, tag)
			slog.Warn("geo: tag not in runetfreedom source", slog.String("scope", "geo"), slog.String("tag", tag))
			continue
		}
		rc, err := f.Open()
		if err != nil {
			skipped = append(skipped, tag)
			slog.Warn("geo: open tag in zip failed", slog.String("scope", "geo"), slog.String("tag", tag), slog.String("err", err.Error()))
			continue
		}
		var b bytes.Buffer
		if _, err := b.ReadFrom(rc); err != nil {
			rc.Close()
			skipped = append(skipped, tag)
			slog.Warn("geo: read tag from zip failed", slog.String("scope", "geo"), slog.String("tag", tag), slog.String("err", err.Error()))
			continue
		}
		rc.Close()
		p, err := m.writeCache(PresetRunetfreedom, tag, b.Bytes())
		if err != nil {
			skipped = append(skipped, tag)
			slog.Warn("geo: tag cache write failed", slog.String("scope", "geo"), slog.String("tag", tag), slog.String("err", err.Error()))
			continue
		}
		out[tag] = p
		slog.Debug("geo: tag downloaded", slog.String("scope", "geo"), slog.String("tag", tag))
	}
	if len(out) == 0 && len(tags) > 0 {
		return nil, fmt.Errorf("geo: no tags available from runetfreedom (skipped %v)", skipped)
	}
	return out, nil
}
