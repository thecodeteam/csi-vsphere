package service

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/thecodeteam/csi-vsphere/pkg/vmware/fs"
	"github.com/thecodeteam/gofsutil"
	"golang.org/x/net/context"
)

func (s *service) GetDevicePath(
	ctx context.Context,
	volID string,
	pubVolInfo, userCreds map[string]string) (string, error) {

	return s.getDevicePathByVolumeID(ctx, volID)
}

func (s *service) GetPrivateMountTargetName(
	ctx context.Context,
	volID string,
	userCreds map[string]string) (string, error) {

	return "", nil
}

func (s *service) FormatAndMount() FormatAndMountFunc {
	return nil
}

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

	if err := gofsutil.EvalSymlinks(ctx, &devicePath); err != nil {
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

func hash(s string) string {
	h := md5.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

func validateDevice(devicePath string) (string, error) {

	if _, err := os.Lstat(devicePath); err != nil {
		return "", err
	}

	// Eval any symlinks to ensure the specified path points to a real device.
	realPath, err := filepath.EvalSymlinks(devicePath)
	if err != nil {
		return "", err
	}
	devicePath = realPath

	if stat, _ := os.Stat(devicePath); stat == nil ||
		stat.Mode()&os.ModeDevice == 0 {
		return "", fmt.Errorf("invalid block device: %s", devicePath)
	}

	return devicePath, nil
}
