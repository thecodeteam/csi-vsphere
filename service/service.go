package service

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"

	"google.golang.org/grpc"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/thecodeteam/gocsi"
	"github.com/thecodeteam/gocsi/csp"
	"github.com/thecodeteam/gofsutil"

	"github.com/thecodeteam/csi-vsphere/pkg/vmware/vmdkops"
)

const (
	// Name is the name of this CSI SP.
	Name = "com.thecodeteam.vsphere"

	// VendorVersion is the version of this CSP SP.
	VendorVersion = "0.1.0"

	// SupportedVersions is a list of the CSI versions this SP supports.
	SupportedVersions = "0.0.0"
)

// Service is a CSI SP and gocsi.IdempotencyProvider.
type Service interface {
	csi.ControllerServer
	csi.IdentityServer
	csi.NodeServer
	gocsi.IdempotencyProvider
	Interceptors() []grpc.UnaryServerInterceptor
	BeforeServe(context.Context, *csp.StoragePlugin, net.Listener) error
}

type service struct {
	ops              vmdkops.VmdkOps
	defaultDatastore string
	privMntDir       string
}

// New returns a new Service.
func New() Service {
	return &service{}
}

func (s *service) BeforeServe(
	ctx context.Context,
	sp *csp.StoragePlugin,
	lis net.Listener) error {

	vmdkops.EsxPort = 1019
	if v, ok := gocsi.LookupEnv(ctx, EnvVarPort); ok {
		i, err := strconv.Atoi(v)
		if err != nil {
			return err
		}
		vmdkops.EsxPort = i
	}

	if v, ok := gocsi.LookupEnv(ctx, EnvVarDatastore); ok {
		s.defaultDatastore = v
	}

	s.ops = vmdkops.VmdkOps{Cmd: vmdkops.EsxVmdkCmd{Mtx: &sync.Mutex{}}}

	privMntDir, privMntDirOk := gocsi.LookupEnv(ctx, csp.EnvVarPrivateMountDir)
	if !privMntDirOk {
		var err error
		if privMntDir, err = filepath.Abs(".csi-vsphere"); err != nil {
			return err
		}
		if err := gofsutil.EvalSymlinks(ctx, &privMntDir); err != nil {
			return err
		}
		if !filepath.IsAbs(privMntDir) {
			return fmt.Errorf(
				"private mount dir must be absolute: %s", privMntDir)
		}
	}
	if err := os.MkdirAll(privMntDir, 0755); err != nil {
		return err
	}
	s.privMntDir = privMntDir

	sp.Interceptors = append(
		sp.Interceptors, NewNodeVolumePublicist(s, privMntDir))
	return nil
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
