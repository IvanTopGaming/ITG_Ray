package clashapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/itg-team/itg-ray/internal/stats"
)

type Client struct {
	base   string
	secret string
	http   *http.Client
}

func New(port int, secret string) *Client {
	return &Client{
		base:   fmt.Sprintf("http://127.0.0.1:%d", port),
		secret: secret,
		http:   &http.Client{Timeout: 3 * time.Second},
	}
}

func (c *Client) Secret() string { return c.secret }

var wireModes = map[string]string{"rule": "Rule", "global": "Global", "direct": "Direct"}

func ValidMode(mode string) bool {
	_, ok := wireModes[mode]
	return ok
}

func (c *Client) Mode(ctx context.Context) (string, error) {
	var out struct {
		Mode string `json:"mode"`
	}
	if err := c.do(ctx, http.MethodGet, "/configs", nil, &out); err != nil {
		return "", err
	}
	return strings.ToLower(out.Mode), nil
}

func (c *Client) SetMode(ctx context.Context, mode string) error {
	wire, ok := wireModes[mode]
	if !ok {
		return fmt.Errorf("clash api: unknown routing mode %q", mode)
	}
	return c.do(ctx, http.MethodPatch, "/configs", map[string]string{"mode": wire}, nil)
}

func (c *Client) CloseConnections(ctx context.Context) error {
	return c.do(ctx, http.MethodDelete, "/connections", nil, nil)
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var rd *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.secret)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("clash api %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("clash api %s %s: status %d", method, path, resp.StatusCode)
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("clash api %s %s: decode: %w", method, path, err)
		}
	}
	return nil
}

func (c *Client) Connections(ctx context.Context) (stats.Snapshot, error) {
	var raw struct {
		Connections []struct {
			ID       string    `json:"id"`
			Upload   uint64    `json:"upload"`
			Download uint64    `json:"download"`
			Start    time.Time `json:"start"`
			Chains   []string  `json:"chains"`
			Rule     string    `json:"rule"`
			Metadata struct {
				Network       string `json:"network"`
				Host          string `json:"host"`
				DestinationIP string `json:"destinationIP"`
				ProcessPath   string `json:"processPath"`
			} `json:"metadata"`
		} `json:"connections"`
	}
	if err := c.do(ctx, http.MethodGet, "/connections", nil, &raw); err != nil {
		return stats.Snapshot{}, err
	}
	snap := stats.Snapshot{Conns: make([]stats.Conn, 0, len(raw.Connections))}
	for _, rc := range raw.Connections {
		snap.Conns = append(snap.Conns, stats.Conn{
			ID:      rc.ID,
			Host:    rc.Metadata.Host,
			DestIP:  rc.Metadata.DestinationIP,
			Process: rc.Metadata.ProcessPath,
			Rule:    rc.Rule,
			Chains:  rc.Chains,
			Up:      rc.Upload,
			Down:    rc.Download,
			Start:   rc.Start,
			Network: rc.Metadata.Network,
		})
	}
	return snap, nil
}
