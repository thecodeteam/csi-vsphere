package service

import (
	"fmt"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *service) Interceptors() []grpc.UnaryServerInterceptor {
	return []grpc.UnaryServerInterceptor{
		s.formatVolumeIDOrName,
		s.singleModeOnly,
	}
}

func (s *service) formatVolumeIDOrName(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler) (interface{}, error) {

	f := func(val *string, datastore string) {
		if datastore != "" && !strings.Contains(*val, "@") {
			*val = fmt.Sprintf("%s@%s", *val, datastore)
		}
	}

	switch treq := req.(type) {
	case *csi.CreateVolumeRequest:
		datastore := s.defaultDatastore
		if v, ok := treq.Parameters["datastore"]; ok && v != "" {
			datastore = v
		}
		f(&treq.Name, datastore)
	case *csi.DeleteVolumeRequest:
		f(&treq.VolumeId, s.defaultDatastore)
	case *csi.ValidateVolumeCapabilitiesRequest:
		f(&treq.VolumeId, s.defaultDatastore)
	case *csi.ControllerPublishVolumeRequest:
		f(&treq.VolumeId, s.defaultDatastore)
	case *csi.ControllerUnpublishVolumeRequest:
		f(&treq.VolumeId, s.defaultDatastore)
	case *csi.NodePublishVolumeRequest:
		f(&treq.VolumeId, s.defaultDatastore)
	case *csi.NodeUnpublishVolumeRequest:
		f(&treq.VolumeId, s.defaultDatastore)
	}

	return handler(ctx, req)
}

func (s *service) singleModeOnly(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler) (interface{}, error) {

	switch treq := req.(type) {
	case *csi.CreateVolumeRequest:
		for _, cap := range treq.VolumeCapabilities {
			if ok, single := isSingleMode(cap); ok && !single {
				return nil,
					status.Errorf(
						codes.InvalidArgument,
						"unsupported access mdoe: %v",
						cap.AccessMode.Mode)
			}
		}
	case *csi.ValidateVolumeCapabilitiesRequest:
		var (
			msg       string
			supported = true
		)
		for _, cap := range treq.VolumeCapabilities {
			if ok, single := isSingleMode(cap); ok && !single {
				supported = false
				msg = fmt.Sprintf(
					"unsupported access mdoe: %v", cap.AccessMode.Mode)
			}
		}
		return &csi.ValidateVolumeCapabilitiesResponse{
			Supported: supported,
			Message:   msg,
		}, nil
	case *csi.ControllerPublishVolumeRequest:
		if ok, single := isSingleMode(treq.VolumeCapability); ok && !single {
			return nil,
				status.Errorf(
					codes.InvalidArgument,
					"unsupported access mode: %v",
					treq.VolumeCapability.AccessMode.Mode)
		}
	}

	return handler(ctx, req)
}
