//go:build windows

// Package svcmgr wraps the Windows Service Control Manager so the user-facing
// CLI (`itgray-cli helper install/uninstall/start/stop/status`) does not need
// to know any winapi.
package svcmgr

import (
	"fmt"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// State is the SCM service state, stringified.
type State string

// Install registers the service with the given binary path and display
// description. Idempotent: returns nil if the service already exists with the
// same binary path.
func Install(name, binPath, desc string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("scm connect: %w", err)
	}
	defer m.Disconnect() //nolint:errcheck // best-effort cleanup

	if s, err := m.OpenService(name); err == nil {
		s.Close() //nolint:errcheck,gosec // idempotent existence check; close is best-effort
		return nil
	}

	cfg := mgr.Config{
		StartType:        mgr.StartAutomatic,
		ErrorControl:     mgr.ErrorNormal,
		BinaryPathName:   binPath,
		DisplayName:      "ITG Ray Helper",
		Description:      desc,
		ServiceStartName: "LocalSystem",
	}
	s, err := m.CreateService(name, binPath, cfg)
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	defer s.Close() //nolint:errcheck // best-effort cleanup
	return nil
}

// Uninstall removes the service. The service must be stopped first.
func Uninstall(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("scm connect: %w", err)
	}
	defer m.Disconnect() //nolint:errcheck // best-effort cleanup
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("open service: %w", err)
	}
	defer s.Close() //nolint:errcheck // best-effort cleanup
	return s.Delete()
}

// Start asks SCM to start the service and waits up to 10 s for Running.
func Start(name string) error {
	return controlAndWait(name, svc.Running, func(s *mgr.Service) error { return s.Start() })
}

// Stop asks SCM to stop the service and waits up to 10 s for Stopped.
func Stop(name string) error {
	return controlAndWait(name, svc.Stopped, func(s *mgr.Service) error {
		_, err := s.Control(svc.Stop)
		return err
	})
}

// Status returns the human-readable current state. Uses minimal SCM
// privileges (SC_MANAGER_CONNECT + SERVICE_QUERY_STATUS) so the GUI
// running as the logged-in standard user can poll it without UAC —
// the higher-privilege Install/Start/Stop go through the elevated
// itgray-cli shell-out instead.
//
// Bypasses mgr.Connect/OpenService because both request _ALL_ACCESS
// rights, which fail with "scm connect: Access is denied" under the
// non-elevated GUI token.
func Status(name string) (State, error) {
	scmHandle, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return "", fmt.Errorf("scm connect: %w", err)
	}
	defer windows.CloseServiceHandle(scmHandle) //nolint:errcheck // best-effort cleanup

	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return "", fmt.Errorf("encode name: %w", err)
	}
	svcHandle, err := windows.OpenService(scmHandle, namePtr, windows.SERVICE_QUERY_STATUS)
	if err != nil {
		return "", fmt.Errorf("open service: %w", err)
	}
	defer windows.CloseServiceHandle(svcHandle) //nolint:errcheck // best-effort cleanup

	var status windows.SERVICE_STATUS
	if err := windows.QueryServiceStatus(svcHandle, &status); err != nil {
		return "", fmt.Errorf("query: %w", err)
	}
	return stateString(svc.State(status.CurrentState)), nil
}

func controlAndWait(name string, want svc.State, action func(*mgr.Service) error) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("scm connect: %w", err)
	}
	defer m.Disconnect() //nolint:errcheck // best-effort cleanup
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("open service: %w", err)
	}
	defer s.Close() //nolint:errcheck // best-effort cleanup
	if err := action(s); err != nil {
		return err
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		st, err := s.Query()
		if err != nil {
			return fmt.Errorf("query: %w", err)
		}
		if st.State == want {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for state %v", want)
}

func stateString(s svc.State) State {
	switch s {
	case svc.Stopped:
		return "Stopped"
	case svc.StartPending:
		return "StartPending"
	case svc.StopPending:
		return "StopPending"
	case svc.Running:
		return "Running"
	case svc.ContinuePending:
		return "ContinuePending"
	case svc.PausePending:
		return "PausePending"
	case svc.Paused:
		return "Paused"
	}
	return "Unknown"
}
