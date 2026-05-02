package svcmgr

import (
	"errors"
	"testing"
)

func TestIsNotInstalled(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"unrelated", errors.New("scm connect: access denied"), false},
		{"lowercase exact", errors.New("service does not exist"), true},
		{"wrapped lowercase", errors.New("open service: service does not exist"), true},
		{"mixed case", errors.New("Open Service: Service Does Not Exist"), true},
		{"alt phrasing", errors.New("the specified service does not exist as an installed service"), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsNotInstalled(tc.err); got != tc.want {
				t.Fatalf("IsNotInstalled(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestIsNotRunning(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"running", errors.New("the requested control is not valid for this service"), false},
		{"not started", errors.New("the service has not been started"), true},
		{"is not started", errors.New("service is not started"), true},
		{"not running", errors.New("ITGRayHelper not running"), true},
		{"mixed case", errors.New("The Service Has Not Been Started"), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsNotRunning(tc.err); got != tc.want {
				t.Fatalf("IsNotRunning(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
