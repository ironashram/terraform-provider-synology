package core

import (
	"context"
	"errors"
	"testing"

	"github.com/synology-community/go-synology/pkg/api"
	synocore "github.com/synology-community/go-synology/pkg/api/core"
)

type fakeShareEnsureAPI struct {
	share       *synocore.ShareGetResponse
	shareErr    error
	volumeResp  *synocore.VolumeListResponse
	volumeErr   error
	createErr   error
	createdWith *synocore.ShareInfo
}

func (f *fakeShareEnsureAPI) ShareGet(
	_ context.Context,
	_ string,
) (*synocore.ShareGetResponse, error) {
	return f.share, f.shareErr
}

func (f *fakeShareEnsureAPI) VolumeList(_ context.Context) (*synocore.VolumeListResponse, error) {
	return f.volumeResp, f.volumeErr
}

func (f *fakeShareEnsureAPI) ShareCreate(_ context.Context, share synocore.ShareInfo) error {
	f.createdWith = &share
	return f.createErr
}

func TestRequireShareExistsReturnsSentinelForMissingShare(t *testing.T) {
	t.Parallel()

	err := RequireShareExists(context.Background(), &fakeShareEnsureAPI{
		shareErr: api.ApiError{Code: 5801},
	}, "media")
	if !errors.Is(err, errShareNotFound) {
		t.Fatalf("expected errShareNotFound, got %v", err)
	}
}

func TestEnsureShareExistsCreatesMissingShare(t *testing.T) {
	t.Parallel()

	client := &fakeShareEnsureAPI{
		shareErr: api.ApiError{Code: 5801},
		volumeResp: &synocore.VolumeListResponse{
			Volumes: []synocore.Volume{{
				VolumePath: "/volume1",
			}},
		},
	}

	if err := EnsureShareExists(context.Background(), client, "media"); err != nil {
		t.Fatalf("EnsureShareExists returned error: %v", err)
	}
	if client.createdWith == nil {
		t.Fatal("expected ShareCreate to be called")
	}
	if client.createdWith.Name != "media" {
		t.Fatalf("share name mismatch: got %q, want %q", client.createdWith.Name, "media")
	}
	if client.createdWith.VolPath != "/volume1" {
		t.Fatalf("volume path mismatch: got %q, want %q", client.createdWith.VolPath, "/volume1")
	}
}

func TestEnsureShareExistsSkipsExistingShare(t *testing.T) {
	t.Parallel()

	client := &fakeShareEnsureAPI{
		share: &synocore.ShareGetResponse{},
	}

	if err := EnsureShareExists(context.Background(), client, "media"); err != nil {
		t.Fatalf("EnsureShareExists returned error: %v", err)
	}
	if client.createdWith != nil {
		t.Fatal("did not expect ShareCreate to be called")
	}
}
