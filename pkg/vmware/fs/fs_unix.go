// Copyright 2016-2017 VMware, Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This is the filesystem interface for mounting volumes on a linux guest.

// +build linux darwin

package fs

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"
)

const (
	sysPciDevs     = "/sys/bus/pci/devices" // All PCI devices on the host
	sysPciSlots    = "/sys/bus/pci/slots"   // PCI slots on the host
	pciAddrLen     = 10                     // Length of PCI dev addr
	diskWatchPath  = "/dev/disk/by-path"
	devWaitTimeout = 10 * time.Second // give it plenty of time to sense the attached disk
)

// DevAttachWaitPrep creates a watcher that watches disk events.
func DevAttachWaitPrep() (*fsnotify.Watcher, error) {
	return devAttachWaitPrep(diskWatchPath)
}

// devAttachWaitPrep creates a watcher that watches devPath.
func devAttachWaitPrep(devPath string) (*fsnotify.Watcher, error) {
	watcher, errWatcher := fsnotify.NewWatcher()
	if errWatcher != nil {
		log.WithFields(log.Fields{"err": errWatcher.Error()}).Error("Failed to create watcher ")
		return nil, errors.New("Failed to create watcher")
	}

	err := watcher.Add(devPath)
	if err != nil {
		log.WithFields(log.Fields{"path": devPath, "err": err.Error()}).Error("Failed to watch ")
		return nil, fmt.Errorf("Failed to watch path %s", devPath)
	}
	return watcher, nil
}

// DevAttachWait waits for attach operation to be completed
func DevAttachWait(
	watcher *fsnotify.Watcher, volDev *VolumeDevSpec) (string, error) {

	device, err := GetDevicePath(volDev)
	if err != nil {
		log.WithFields(
			log.Fields{"volDev": *volDev, "err": err}).Error(
			"Failed to get device path ")
		return "", err
	}

	// This file is created after a device is attached to VM
	// and removed when device is detached.
	// If file already present, do not wait for attach
	_, err = os.Stat(device)
	if err == nil {
		log.WithField("device", device).Info("device file found")
		return "", nil
	}

	devAttachWait(watcher, device)
	return device, nil
}

// devAttachWait waits for attach operation to be completed
func devAttachWait(watcher *fsnotify.Watcher, device string) {
loop:
	for {
		select {
		case ev := <-watcher.Events:
			log.Debug("event: ", ev)
			if ev.Name == device {
				// Log when the device is discovered
				log.WithFields(
					log.Fields{"device": device, "event": ev},
				).Info("Scan complete ")
				break loop
			}
		case err := <-watcher.Errors:
			log.WithFields(
				log.Fields{"device": device, "error": err},
			).Error("Hit error during watch ")
			break loop
		case <-time.After(devWaitTimeout):
			log.WithFields(
				log.Fields{"timeout": devWaitTimeout, "device": device},
			).Warning("Exceeded timeout while waiting for device attach to complete")
			break loop
		}
	}
	watcher.Close()
}

// GetDevicePath returns the device path or error.
func GetDevicePath(volDev *VolumeDevSpec) (string, error) {
	if volDev.ControllerPciBusNumber == "" {
		return getDevicePathNoBus(volDev)
	}
	return getDevicePath(volDev)
}

func getDevicePath(volDev *VolumeDevSpec) (string, error) {
	// The bus device number in the volume dev spec refers the PCI
	// bridge under which the device is attached. Read the pci_bus dir
	// under /sys/bus/pci/devices/<bridge>/pci_bus to get the device
	// (PVSCSI controller) we are interested in.
	pciBusDir := fmt.Sprintf("%s/0000:00:%s/pci_bus", sysPciDevs,
		volDev.ControllerPciBusNumber)

	devs, err := ioutil.ReadDir(pciBusDir)
	if err != nil {
		log.WithFields(log.Fields{"Error": err}).Warn("Get device path failed for unit %s @ PCI bridge device %s: ",
			volDev.Unit, volDev.ControllerPciBusNumber)
		return "", fmt.Errorf("Device not found")
	}
	// This assumes that there is only one controller attached to the PCI bridge (which
	// is the case anyway with PCIE).
	pvscsiCtl := fmt.Sprintf("%s:00.0", devs[0].Name())
	return fmt.Sprintf("/dev/disk/by-path/pci-%s-scsi-0:0:%s:0", pvscsiCtl, volDev.Unit), nil
}

func getDevicePathNoBus(volDev *VolumeDevSpec) (string, error) {
	// Get the device node for the unit returned from the attach.
	// Lookup each device that has a label and if that label matches
	// the one for the given bus number.
	// The device we need is then constructed from the dir name with
	// the matching label.
	pciSlotAddr := fmt.Sprintf("%s/%s/address", sysPciSlots, volDev.ControllerPciSlotNumber)

	fh, err := os.Open(pciSlotAddr)
	if err != nil {
		log.WithFields(log.Fields{"Error": err}).Warn("Get device path failed for unit# %s @ PCI slot %s: ",
			volDev.Unit, volDev.ControllerPciSlotNumber)
		return "", fmt.Errorf("Device not found")
	}

	buf := make([]byte, pciAddrLen)
	_, err = fh.Read(buf)

	fh.Close()
	if err != nil && err != io.EOF {
		log.WithFields(log.Fields{"Error": err}).Warn("Get device path failed for unit# %s @ PCI slot %s: ",
			volDev.Unit, volDev.ControllerPciSlotNumber)
		return "", fmt.Errorf("Device not found")
	}
	return fmt.Sprintf("/dev/disk/by-path/pci-%s.0-scsi-0:0:%s:0", string(buf), volDev.Unit), nil
}
