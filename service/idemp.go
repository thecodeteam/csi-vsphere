package service

import (
	"context"
	"path"

	"github.com/container-storage-interface/spec/lib/go/csi"

	"github.com/thecodeteam/gofsutil"
)

func (s *service) GetVolumeID(
	ctx context.Context,
	name string) (string, error) {

	if vol, err := s.toVolumeInfo(name); err == nil {
		return vol.Id, nil
	}
	return "", nil
}

func (s *service) GetVolumeInfo(
	ctx context.Context,
	id, name string) (*csi.VolumeInfo, error) {

	if id == "" {
		id = name
	}

	if vol, err := s.toVolumeInfo(id); err == nil {
		return &vol, nil
	}

	return nil, nil
}

func (s *service) IsControllerPublished(
	ctx context.Context,
	id, nodeID string) (map[string]string, error) {

	devicePath, err := s.getDevicePathByVolumeID(ctx, id)
	if err != nil {
		return nil, err
	}
	if devicePath == "" {
		return nil, err
	}
	return map[string]string{"device": devicePath}, nil
}

func (s *service) IsNodePublished(
	ctx context.Context,
	id string,
	pubInfo map[string]string,
	targetPath string) (bool, error) {

	privMntTgtPath := path.Join(s.privMntDir, hash(id))
	mountTable, err := gofsutil.GetMounts(ctx)
	if err != nil {
		return false, err
	}

	for _, m := range mountTable {
		if m.Source == privMntTgtPath && m.Path == targetPath {
			return true, nil
		}
	}

	return false, nil
}
