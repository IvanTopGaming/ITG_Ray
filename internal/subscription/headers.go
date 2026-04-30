package subscription

import (
	"encoding/base64"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Userinfo holds quota figures parsed from the Subscription-Userinfo header.
// The Has* flags distinguish "field absent or malformed in the header" from
// "field explicitly set to zero" — without them, a partial header would
// silently overwrite stored quota values with zeros on UpdateMeta.
type Userinfo struct {
	Upload      int64
	Download    int64
	Total       int64
	Expire      *time.Time
	HasUpload   bool
	HasDownload bool
	HasTotal    bool
}

// Headers holds the de-facto-standard subscription metadata that panels emit
// (Subscription-Userinfo, profile-update-interval, profile-title).
type Headers struct {
	Userinfo       *Userinfo
	UpdateInterval int // hours
	ProfileTitle   string
}

// ParseHeaders extracts the standard subscription metadata from an HTTP response.
// Missing or malformed headers result in zero values rather than errors.
func ParseHeaders(h http.Header) Headers {
	out := Headers{}
	if s := h.Get("Subscription-Userinfo"); s != "" {
		out.Userinfo = parseUserinfo(s)
	}
	if s := h.Get("profile-update-interval"); s != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
			out.UpdateInterval = n
		} else {
			slog.Debug("profile-update-interval: skipping non-numeric", slog.String("scope", "subscription.headers"), slog.String("value", s))
		}
	}
	if s := h.Get("profile-title"); s != "" {
		if dec, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s)); err == nil {
			out.ProfileTitle = string(dec)
		} else {
			out.ProfileTitle = s
		}
	}
	return out
}

func parseUserinfo(s string) *Userinfo {
	u := &Userinfo{}
	for _, part := range strings.Split(s, ";") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			slog.Debug("subscription-userinfo: skipping malformed entry", slog.String("scope", "subscription.headers"), slog.String("entry", part))
			continue
		}
		v, err := strconv.ParseInt(strings.TrimSpace(kv[1]), 10, 64)
		if err != nil {
			slog.Debug("subscription-userinfo: skipping non-numeric value", slog.String("scope", "subscription.headers"), slog.String("key", kv[0]), slog.String("value", kv[1]))
			continue
		}
		switch strings.TrimSpace(kv[0]) {
		case "upload":
			u.Upload = v
			u.HasUpload = true
		case "download":
			u.Download = v
			u.HasDownload = true
		case "total":
			u.Total = v
			u.HasTotal = true
		case "expire":
			// expire=0 is the Subscription-Userinfo convention for "no expiry";
			// don't store a real timestamp at the Unix epoch.
			if v <= 0 {
				continue
			}
			t := time.Unix(v, 0)
			u.Expire = &t
		}
	}
	return u
}
