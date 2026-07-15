package filestation

import (
	"errors"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
	gofs "github.com/synology-community/go-synology/pkg/api/filestation"
)

func TestFileStationPathWaitTimeoutDefault(t *testing.T) {
	got := fileStationPathWaitTimeout(true, types.Int64Null())
	if got != 300*time.Second {
		t.Fatalf("expected default timeout of 300s, got %s", got)
	}
}

func TestFileStationPathWaitTimeoutExplicit(t *testing.T) {
	got := fileStationPathWaitTimeout(true, types.Int64Value(42))
	if got != 42*time.Second {
		t.Fatalf("expected explicit timeout of 42s, got %s", got)
	}
}

func TestFileStationPathWaitTimeoutDisabled(t *testing.T) {
	got := fileStationPathWaitTimeout(false, types.Int64Value(42))
	if got != 0 {
		t.Fatalf("expected zero timeout when waiting is disabled, got %s", got)
	}
}

func TestIsFileStationNotFound(t *testing.T) {
	if !isFileStationNotFound(gofs.FileNotFoundError{Path: "/missing"}) {
		t.Fatal("expected FileNotFoundError to be treated as not found")
	}

	if !isFileStationNotFound(errors.New("result is empty")) {
		t.Fatal("expected result-is-empty error to be treated as not found")
	}

	if isFileStationNotFound(errors.New("permission denied")) {
		t.Fatal("expected unrelated error to not be treated as not found")
	}
}
