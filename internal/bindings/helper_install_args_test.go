package bindings

import (
	"reflect"
	"testing"
)

func TestInstallHelperArgs(t *testing.T) {
	name, args := installHelperArgs("/tmp/stage/itgray-helper", 1000)
	if name != "pkexec" {
		t.Fatalf("name = %q, want pkexec", name)
	}
	want := []string{"/tmp/stage/itgray-helper", "install", "--uid", "1000"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
}
