package geo

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
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
	for _, tag := range missing {
		var f *zip.File
		for _, n := range sourceNames(tag) {
			if hit, ok := index[runetfreedomZipEntry(n)]; ok {
				f = hit
				break
			}
		}
		if f == nil {
			return nil, fmt.Errorf("geo: tag %q not found in runetfreedom source", tag)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("geo: open %q in zip: %w", tag, err)
		}
		var b bytes.Buffer
		if _, err := b.ReadFrom(rc); err != nil {
			rc.Close()
			return nil, fmt.Errorf("geo: read %q from zip: %w", tag, err)
		}
		rc.Close()
		p, err := m.writeCache(PresetRunetfreedom, tag, b.Bytes())
		if err != nil {
			return nil, fmt.Errorf("geo: cache %q: %w", tag, err)
		}
		out[tag] = p
	}
	return out, nil
}
