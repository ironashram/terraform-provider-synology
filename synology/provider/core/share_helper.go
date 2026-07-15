package core

import (
	"context"
	"errors"
	"fmt"

	"github.com/synology-community/go-synology/pkg/api"
	synocore "github.com/synology-community/go-synology/pkg/api/core"
)

var errShareNotFound = errors.New("synology share not found")

type shareLookupAPI interface {
	ShareGet(ctx context.Context, name string) (*synocore.ShareGetResponse, error)
}

type shareEnsureAPI interface {
	shareLookupAPI
	VolumeList(ctx context.Context) (*synocore.VolumeListResponse, error)
	ShareCreate(ctx context.Context, share synocore.ShareInfo) error
}

func LookupShareByName(
	ctx context.Context,
	client shareLookupAPI,
	shareName string,
) (*synocore.ShareGetResponse, error) {
	share, err := client.ShareGet(ctx, shareName)
	if err != nil {
		if IsMissingShareAPIError(err) {
			return nil, errShareNotFound
		}
		return nil, err
	}

	return share, nil
}

func RequireShareExists(ctx context.Context, client shareLookupAPI, shareName string) error {
	_, err := LookupShareByName(ctx, client, shareName)
	return err
}

func EnsureShareExists(ctx context.Context, client shareEnsureAPI, shareName string) error {
	_, err := LookupShareByName(ctx, client, shareName)
	switch {
	case err == nil:
		return nil
	case !errors.Is(err, errShareNotFound):
		return err
	}

	volumes, err := client.VolumeList(ctx)
	if err != nil {
		return err
	}
	if len(volumes.Volumes) == 0 {
		return fmt.Errorf("no DSM volume found for shared folder %q", shareName)
	}

	return client.ShareCreate(ctx, synocore.ShareInfo{
		Name:    shareName,
		VolPath: volumes.Volumes[0].VolumePath,
	})
}

func IsMissingShareAPIError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, errShareNotFound) {
		return true
	}

	var notFound api.NotFoundError
	if errors.As(err, &notFound) {
		return true
	}

	var apiErr api.ApiError
	if errors.As(err, &apiErr) {
		switch apiErr.Code {
		case 3551, 5801, 6013:
			return true
		}
	}

	return false
}
