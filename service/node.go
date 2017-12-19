package service

import (
	"fmt"
	"os"
	"path"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/thecodeteam/csi-vsphere/pkg/vmware/fs"
	"github.com/thecodeteam/gocsi"
	"github.com/thecodeteam/gofsutil"
)

func (s *service) NodePublishVolume(
	ctx context.Context,
	req *csi.NodePublishVolumeRequest) (
	*csi.NodePublishVolumeResponse, error) {

	var (
		isBlock    bool
		fsType     string
		mntFlags   []string
		readOnly   = req.Readonly
		accessMode = req.VolumeCapability.AccessMode.Mode
	)

	lf := log.Fields{
		"volumeID":   req.VolumeId,
		"targetPath": req.TargetPath,
		"readOnly":   req.Readonly,
		"accessMode": accessMode,
	}

	switch accessType := req.VolumeCapability.AccessType.(type) {
	case *csi.VolumeCapability_Block:
		if accessType.Block == nil {
			return nil, gocsi.ErrBlockTypeRequired
		}
		if readOnly {
			return nil, status.Error(
				codes.InvalidArgument,
				"read only not supported by access type")
		}
		isBlock = true
		lf["accessType"] = "block"
	case *csi.VolumeCapability_Mount:
		if accessType.Mount == nil {
			return nil, gocsi.ErrMountTypeRequired
		}
		fsType = accessType.Mount.FsType
		mntFlags = accessType.Mount.MountFlags
		if readOnly {
			hasROMntFlag := false
			for _, o := range mntFlags {
				if o == "ro" {
					hasROMntFlag = true
					break
				}
			}
			if !hasROMntFlag {
				mntFlags = append(mntFlags, "ro")
			}
		}
		lf["accessType"] = "mount"
		lf["fsType"] = fsType
		lf["mntFlags"] = mntFlags
	default:
		return nil, gocsi.ErrAccessTypeRequired
	}

	// Attach the VMDK to this node.
	dev, err := s.ops.Attach(req.VolumeId, nil)
	if err != nil {
		return nil, err
	}

	// Create a watcher object used to wait for the VMDK to be attached.
	watcher, err := fs.DevAttachWaitPrep()
	if err != nil {
		return nil, err
	}

	// Wait for the VMDK to be attached as a device to this node.
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

	validatedDevPath, err := gofsutil.ValidateDevice(ctx, devicePath)
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}
	devicePath = validatedDevPath
	lf["devicePath"] = devicePath

	privMntTgtPath := path.Join(s.privMntDir, hash(req.VolumeId))
	lf["privateMountTargetPath"] = privMntTgtPath

	mountTable, err := gofsutil.GetMounts(ctx)
	if err != nil {
		return nil, err
	}

	var (
		isPub     bool
		isPrivPub bool
	)

	for _, m := range mountTable {
		// Check to see if the device is already mounted to the private
		// target path.
		if m.Source == devicePath && m.Path == privMntTgtPath {
			stat, _ := os.Stat(privMntTgtPath)
			if stat == nil {
				continue
			}
			if (isBlock && stat.IsDir()) || !stat.IsDir() {
				return nil, fmt.Errorf(
					"invalid existing private mount target: %s",
					privMntTgtPath)
			}
			isPrivPub = true
			lf["deviceToPrivate"] = isPrivPub
		}

		// Check to see if the private target path is already mounted to the
		// target path.
		if m.Source == privMntTgtPath && m.Path == req.TargetPath {
			stat, _ := os.Stat(req.TargetPath)
			if stat == nil {
				continue
			}
			if (isBlock && stat.IsDir()) || !stat.IsDir() {
				return nil, fmt.Errorf(
					"invalid existing target path: %s",
					req.TargetPath)
			}
			isPub = true
			lf["privateToPublic"] = isPub
		}
	}

	if !isPrivPub {
		if isBlock {
			if _, err := os.Stat(privMntTgtPath); err != nil {
				if !os.IsNotExist(err) {
					return nil, err
				}
				log.WithFields(lf).Debug("creating private mount target file")
				f, err := os.Create(privMntTgtPath)
				if err != nil {
					return nil, err
				}
				f.Close()
			}
			log.WithFields(lf).Debug(
				"bind mounting device to private mount target")

			if err := gofsutil.BindMount(
				ctx, devicePath, privMntTgtPath); err != nil {
				return nil, err
			}
		} else {
			if _, err := os.Stat(privMntTgtPath); err != nil {
				if !os.IsNotExist(err) {
					return nil, err
				}
				log.WithFields(lf).Debug("creating private mount target dir")
				if err := os.MkdirAll(privMntTgtPath, 0755); err != nil {
					return nil, err
				}
			}
			log.WithFields(lf).Debug(
				"formatting device & mounting to private mount target")

			err := gofsutil.FormatAndMount(
				ctx, devicePath, privMntTgtPath, fsType, mntFlags...)
			if err != nil {
				return nil, err
			}
		}
	}

	if !isPub {
		log.WithFields(lf).Debug(
			"bind mounting private mount target to target path")
		err := gofsutil.BindMount(ctx, privMntTgtPath, req.TargetPath)
		if err != nil {
			return nil, err
		}
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

func (s *service) NodeUnpublishVolume(
	ctx context.Context,
	req *csi.NodeUnpublishVolumeRequest) (
	*csi.NodeUnpublishVolumeResponse, error) {

	lf := log.Fields{
		"volumeID":   req.VolumeId,
		"targetPath": req.TargetPath,
	}

	privMntTgtPath := path.Join(s.privMntDir, hash(req.VolumeId))
	lf["privateMountTargetPath"] = privMntTgtPath

	privMntTgtStat, err := os.Stat(privMntTgtPath)
	if err != nil {
		return nil, err
	}
	isBlock := !privMntTgtStat.IsDir()
	lf["isBlock"] = isBlock

	mountTable, err := gofsutil.GetMounts(ctx)
	if err != nil {
		return nil, err
	}

	var (
		mountPoints     int
		isTargetMounted bool
	)

	for _, m := range mountTable {
		if m.Source == privMntTgtPath {
			mountPoints++
			if m.Path == req.TargetPath {
				isTargetMounted = true
				lf["isTargetMounted"] = true
			}
		}
	}

	if isTargetMounted {
		log.WithFields(lf).Debug("unmounting target path")
		if err := gofsutil.Unmount(ctx, req.TargetPath); err != nil {
			return nil, err
		}
	}

	// If the private mount path was only bind mounted once it means
	// that the private mount should be unmounted.
	if mountPoints == 1 {
		log.WithFields(lf).Debug("unmounting private mount target path")
		if err := gofsutil.Unmount(ctx, privMntTgtPath); err != nil {
			return nil, err
		}
		if err := os.RemoveAll(privMntTgtPath); err != nil {
			return nil, err
		}

		// Detach the VMDK from this host.
		devicePath, err := s.getDevicePathByVolumeID(ctx, req.VolumeId)
		if err != nil {
			return nil, err
		}
		lf["devicePath"] = devicePath
		log.WithFields(lf).Debug("detaching vmdk device")
		if err := s.ops.Detach(req.VolumeId, nil); err != nil {
			return nil, err
		}
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (s *service) GetNodeID(
	ctx context.Context,
	req *csi.GetNodeIDRequest) (
	*csi.GetNodeIDResponse, error) {

	return &csi.GetNodeIDResponse{NodeId: Name}, nil
}

func (s *service) NodeProbe(
	ctx context.Context,
	req *csi.NodeProbeRequest) (
	*csi.NodeProbeResponse, error) {

	if _, err := s.ops.List(); err != nil {
		return nil, err
	}
	return &csi.NodeProbeResponse{}, nil
}

func (s *service) NodeGetCapabilities(
	ctx context.Context,
	req *csi.NodeGetCapabilitiesRequest) (
	*csi.NodeGetCapabilitiesResponse, error) {

	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: []*csi.NodeServiceCapability{
			&csi.NodeServiceCapability{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_UNKNOWN,
					},
				},
			},
		},
	}, nil
}
