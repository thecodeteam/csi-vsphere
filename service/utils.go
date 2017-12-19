package service

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"regexp"
	"strconv"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/thecodeteam/csi-vsphere/pkg/vmware/fs"
	"github.com/thecodeteam/gofsutil"
)

func (s *service) getDevicePathByVolumeID(
	ctx context.Context, id string) (string, error) {

	devInfo, ok, err := s.getDeviceInfo(ctx, id)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", nil
	}

	devicePath, err := fs.GetDevicePath(&devInfo)
	if err != nil {
		return "", err
	}
	if devicePath == "" {
		return "", nil
	}

	if devicePath, err = gofsutil.ValidateDevice(ctx, devicePath); err != nil {
		return "", err
	}

	return devicePath, nil
}

func (s *service) getDeviceInfo(
	ctx context.Context, id string) (fs.VolumeDevSpec, bool, error) {

	var devInfo fs.VolumeDevSpec

	data, err := s.ops.Get(id)
	if err != nil {
		return devInfo, false, err
	}

	if st, ok := data["status"].(string); !ok || st != "attached" {
		return devInfo, false, nil
	}

	dev, ok := data["attachedVMDevice"].(map[string]interface{})
	if !ok {
		return devInfo, false, nil
	}

	if len(dev) == 0 {
		return devInfo, false, nil
	}

	unit, _ := dev["Unit"].(string)
	slot, _ := dev["ControllerPciSlotNumber"].(string)
	bus, _ := dev["ControllerPciBusNumber"].(string)

	if unit == "" && slot == "" {
		return devInfo, false, nil
	}

	devInfo.Unit = unit
	devInfo.ControllerPciBusNumber = bus
	devInfo.ControllerPciSlotNumber = slot

	return devInfo, true, nil
}

func (s *service) toVolumeInfo(id string) (csi.VolumeInfo, error) {

	var vol csi.VolumeInfo

	data, err := s.ops.Get(id)
	if err != nil {
		return vol, err
	}

	vol.Id = id
	vol.Attributes = map[string]string{}

	for k, v := range data {
		if k == "capacity" {
			if v, ok := v.(map[string]interface{}); ok {
				if v, ok := v["size"].(string); ok {
					if i, ok := isGB(v); ok {
						vol.CapacityBytes = i * 1024 * 1024 * 1024
					} else if i, ok := isMB(v); ok {
						vol.CapacityBytes = i * 1024 * 1024
					} else if i, ok := isKB(v); ok {
						vol.CapacityBytes = i * 1024
					}
				}
			}
		} else if k == "attachedVMDevice" {
			if v, ok := v.(map[string]interface{}); ok {
				for k, v := range v {
					if v, ok := v.(string); ok && v != "" {
						vol.Attributes[k] = v
					}
				}
			}
		} else if v, ok := v.(string); ok {
			vol.Attributes[k] = v
		}
	}

	if len(vol.Attributes) == 0 {
		vol.Attributes = nil
	}

	return vol, nil
}

func isKB(s string) (uint64, bool) {
	return isSize(`(?i)^([\d,\.]+)\s*KB\s*$`, s)
}

func isMB(s string) (uint64, bool) {
	return isSize(`(?i)^([\d,\.]+)\s*MB\s*$`, s)
}

func isGB(s string) (uint64, bool) {
	return isSize(`(?i)^([\d,\.]+)\s*GB\s*$`, s)
}

func isSize(patt, s string) (uint64, bool) {
	rx := regexp.MustCompile(patt)
	m := rx.FindStringSubmatch(s)
	if len(m) == 0 {
		return 0, false
	}
	i, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, false
	}
	return uint64(i), true
}

func isSingleMode(cap *csi.VolumeCapability) (bool, bool) {
	if cap == nil || cap.AccessMode == nil {
		return false, false
	}
	mode := cap.AccessMode.Mode
	return true, mode == csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER ||
		mode == csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY
}

func hash(s string) string {
	h := md5.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}
