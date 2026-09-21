package subscription

import (
	"errors"
	"log/slog"
	"path/filepath"
	"sync"

	"github.com/itg-team/itg-ray/internal/config"
	"github.com/itg-team/itg-ray/internal/hwid"
)

type InputResolver func(Stored) (Subscription, error)

func NewInputResolver(dataDir, version string) InputResolver {
	if version == "" {
		version = "dev"
	}
	var once sync.Once
	var deviceID string
	var info hwid.DeviceInfo
	return func(sub Stored) (Subscription, error) {
		cfg, err := config.Load(filepath.Join(dataDir, "config.json"))
		if err != nil {
			return Subscription{}, err
		}
		settings := cfg.Subscriptions
		var requestID string
		var requestInfo hwid.DeviceInfo
		if settings.UserAgent == "" {
			settings.UserAgent = "ITGRay/" + version
		}
		if settings.HWIDEnabled {
			once.Do(func() {
				var idErr error
				deviceID, idErr = hwid.Get(dataDir)
				if idErr != nil {
					slog.Warn("subscription identity cache warning", "err", idErr)
				}
				info = hwid.Info()
			})
			requestID, requestInfo = deviceID, info
			if requestID == "" {
				return Subscription{}, errors.New("subscription HWID is unavailable")
			}
		}
		input := sub.ToSyncInput()
		input.UserAgent, input.HWID, input.DeviceOS, input.OSVersion, input.DeviceModel = resolveIdentity(settings, sub, requestID, requestInfo)
		return input, nil
	}
}

func resolveIdentity(settings config.Subscriptions, sub Stored, deviceID string, info hwid.DeviceInfo) (ua, id, os, version, model string) {
	ua = sub.UserAgent
	if ua == "" {
		ua = settings.UserAgent
	}
	if ua == "" {
		ua = "ITGRay/dev"
	}
	if !settings.HWIDEnabled {
		return ua, "", "", "", ""
	}
	id = deviceID
	if settings.SendDeviceOS {
		os = info.OS
	}
	if settings.SendOSVersion {
		version = info.Version
	}
	if settings.SendDeviceModel {
		model = info.Model
	}
	return
}
