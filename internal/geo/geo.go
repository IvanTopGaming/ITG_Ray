package geo

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type Source struct {
	Preset    string
	CustomURL string
}

const (
	PresetRunetfreedom = "runetfreedom"
	PresetSagerNet     = "sagernet"
	PresetCustom       = "custom"

	DefaultSagerNetBase = "https://raw.githubusercontent.com/SagerNet"

	fetchTimeout = 3 * time.Minute
)

type Manager struct {
	DataDir        string
	Client         *http.Client
	Progress       func(done, total int64)
	zipURLOverride string
}

func NewManager(dataDir string, progress func(done, total int64)) *Manager {
	return &Manager{DataDir: dataDir, Client: &http.Client{}, Progress: progress}
}

func (m *Manager) client() *http.Client {
	if m.Client != nil {
		return m.Client
	}
	return http.DefaultClient
}

func (m *Manager) report(done, total int64) {
	if m.Progress != nil {
		m.Progress(done, total)
	}
}

func presetDir(preset string) string {
	switch preset {
	case PresetRunetfreedom, PresetSagerNet, PresetCustom:
		return preset
	default:
		return PresetRunetfreedom
	}
}

func (m *Manager) cachePath(preset, tag string) string {
	return filepath.Join(m.DataDir, "geo", presetDir(preset), tag+".srs")
}

func (m *Manager) cached(preset, tag string) (string, bool) {
	p := m.cachePath(preset, tag)
	if st, err := os.Stat(p); err == nil && st.Size() > 0 {
		return p, true
	}
	return "", false
}

func (m *Manager) writeCache(preset, tag string, data []byte) (string, error) {
	p := m.cachePath(preset, tag)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return "", err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, p); err != nil {
		return "", err
	}
	return p, nil
}

func (m *Manager) Resolve(ctx context.Context, src Source, tags []string) (map[string]string, error) {
	return m.fetch(ctx, src, tags, false)
}

func (m *Manager) Refresh(ctx context.Context, src Source, tags []string) error {
	_, err := m.fetch(ctx, src, tags, true)
	return err
}

func (m *Manager) fetch(ctx context.Context, src Source, tags []string, force bool) (map[string]string, error) {
	if len(tags) == 0 {
		return map[string]string{}, nil
	}
	switch src.Preset {
	case PresetRunetfreedom:
		return m.fetchRunetfreedom(ctx, tags, force)
	case PresetSagerNet:
		return m.fetchDirect(ctx, PresetSagerNet, DefaultSagerNetBase, tags, force)
	case PresetCustom:
		if src.CustomURL == "" {
			return nil, fmt.Errorf("geo: custom source selected but customURL is empty")
		}
		return m.fetchDirect(ctx, PresetCustom, src.CustomURL, tags, force)
	default:
		return m.fetchRunetfreedom(ctx, tags, force)
	}
}

func (m *Manager) fetchRunetfreedom(ctx context.Context, tags []string, force bool) (map[string]string, error) {
	return nil, fmt.Errorf("geo: runetfreedom provider not implemented")
}
