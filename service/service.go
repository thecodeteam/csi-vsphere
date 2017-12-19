package service

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	log "github.com/sirupsen/logrus"
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
	SupportedVersions = "0.0.0, 0.1.0"
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

	privMntDir, _ := gocsi.LookupEnv(ctx, csp.EnvVarPrivateMountDir)
	if privMntDir == "" {
		var err error
		if privMntDir, err = filepath.Abs(".csi-vsphere"); err != nil {
			return err
		}
	}
	if !filepath.IsAbs(privMntDir) {
		return fmt.Errorf(
			"private mount dir must be absolute: %s", privMntDir)
	}
	if err := os.MkdirAll(privMntDir, 0755); err != nil {
		return err
	}
	if err := gofsutil.EvalSymlinks(ctx, &privMntDir); err != nil {
		return err
	}
	s.privMntDir = privMntDir

	log.WithFields(log.Fields{
		"privateMountDir": s.privMntDir,
		"esxPort":         vmdkops.EsxPort,
	}).Infof("configured %s", Name)

	return nil
}
