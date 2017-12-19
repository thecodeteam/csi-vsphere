package service

import (
	"fmt"
	"path/filepath"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/container-storage-interface/spec/lib/go/csi"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"

	"github.com/thecodeteam/csi-vsphere/pkg/vmware/fs"
)

func (s *service) CreateVolume(
	ctx context.Context,
	req *csi.CreateVolumeRequest) (
	*csi.CreateVolumeResponse, error) {

	var size uint64
	opts := map[string]string{}
	if cr := req.CapacityRange; cr != nil {
		if cr.LimitBytes > 0 || cr.RequiredBytes > 0 {
			size = cr.LimitBytes
			if cr.RequiredBytes > size {
				size = cr.RequiredBytes
			}
		}
		if size > 0 {
			mib := size / 1024 / 1024
			opts["size"] = fmt.Sprintf("%dmb", mib)
		}
	}

	log.WithFields(log.Fields{
		"name": req.Name,
		"size": size,
	}).Debug("creating volume")

	if err := s.ops.Create(req.Name, opts); err != nil {
		return nil, err
	}

	vol, err := s.toVolumeInfo(req.Name)
	if err != nil {
		return nil, err
	}

	return &csi.CreateVolumeResponse{VolumeInfo: &vol}, nil
}

func (s *service) DeleteVolume(
	ctx context.Context,
	req *csi.DeleteVolumeRequest) (
	*csi.DeleteVolumeResponse, error) {

	if err := s.ops.Remove(req.VolumeId, nil); err != nil {
		return nil, err
	}
	return &csi.DeleteVolumeResponse{}, nil
}

func (s *service) ControllerPublishVolume(
	ctx context.Context,
	req *csi.ControllerPublishVolumeRequest) (
	*csi.ControllerPublishVolumeResponse, error) {

	dev, err := s.ops.Attach(req.VolumeId, nil)
	if err != nil {
		return nil, err
	}

	watcher, err := fs.DevAttachWaitPrep()
	if err != nil {
		return nil, err
	}

	devicePath, err := fs.DevAttachWait(watcher, dev)
	if err != nil {
		return nil, err
	}
	if devicePath == "" {
		dp, err := fs.GetDevicePath(dev)
		if err != nil {
			return nil, err
		}
		devicePath = dp
	}

	dp, err := filepath.EvalSymlinks(devicePath)
	if err != nil {
		return nil, err
	}
	devicePath = dp

	return &csi.ControllerPublishVolumeResponse{
		PublishVolumeInfo: map[string]string{"device": devicePath},
	}, nil
}

func (s *service) ControllerUnpublishVolume(
	ctx context.Context,
	req *csi.ControllerUnpublishVolumeRequest) (
	*csi.ControllerUnpublishVolumeResponse, error) {

	if err := s.ops.Detach(req.VolumeId, nil); err != nil {
		return nil, err
	}
	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

func (s *service) ValidateVolumeCapabilities(
	ctx context.Context,
	req *csi.ValidateVolumeCapabilitiesRequest) (
	*csi.ValidateVolumeCapabilitiesResponse, error) {

	return nil, nil
}

func (s *service) ListVolumes(
	ctx context.Context,
	req *csi.ListVolumesRequest) (
	*csi.ListVolumesResponse, error) {

	vols, err := s.listVolumes(ctx)
	if err != nil {
		return nil, err
	}

	rep := &csi.ListVolumesResponse{
		Entries: make([]*csi.ListVolumesResponse_Entry, len(vols)),
	}

	for i := range vols {
		rep.Entries[i] = &csi.ListVolumesResponse_Entry{
			VolumeInfo: &vols[i],
		}
	}

	return rep, nil
}

func (s *service) listVolumes(ctx context.Context) ([]csi.VolumeInfo, error) {

	data, err := s.ops.List()
	if err != nil {
		return nil, err
	}

	vols := make([]csi.VolumeInfo, len(data))
	for i, v := range data {
		volInfo, err := s.toVolumeInfo(v.Name)
		if err != nil {
			return nil, err
		}
		vols[i] = volInfo
	}

	return vols, nil
}

func (s *service) GetCapacity(
	ctx context.Context,
	req *csi.GetCapacityRequest) (
	*csi.GetCapacityResponse, error) {

	return nil, status.Error(codes.Unimplemented, "")
}

func (s *service) ControllerGetCapabilities(
	ctx context.Context,
	req *csi.ControllerGetCapabilitiesRequest) (
	*csi.ControllerGetCapabilitiesResponse, error) {

	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: []*csi.ControllerServiceCapability{
			&csi.ControllerServiceCapability{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
					},
				},
			},
			&csi.ControllerServiceCapability{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{
						Type: csi.ControllerServiceCapability_RPC_LIST_VOLUMES,
					},
				},
			},
		},
	}, nil
}

func (s *service) ControllerProbe(
	ctx context.Context,
	req *csi.ControllerProbeRequest) (
	*csi.ControllerProbeResponse, error) {

	if _, err := s.ops.List(); err != nil {
		return nil, err
	}
	return &csi.ControllerProbeResponse{}, nil
}
