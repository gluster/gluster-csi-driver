package glustervirtblock

import (
	"time"

	"github.com/gluster/gluster-csi-driver/pkg/gluster-virtblock/config"

	"github.com/gluster/glusterd2/pkg/restclient"
	"github.com/kubernetes-csi/drivers/pkg/csi-common"
	"k8s.io/klog"
)

const (
	glusterBlockCSIDriverName    = "org.gluster.glustervirtblock"
	glusterBlockCSIDriverVersion = "1.0.0"
)

// GfDriver is the struct embedding information about the connection to gluster cluster and configuration of CSI driver.
type GfDriver struct {
	client *restclient.Client
	*config.Config
}

// New returns CSI driver
func New(config *config.Config) *GfDriver {
	gfd := &GfDriver{}

	if config == nil {
		klog.Errorf("GlusterFS Block CSI driver initialization failed")
		return nil
	}

	gfd.Config = config
	var err error
	gfd.client, err = restclient.NewClientWithOpts(
		restclient.WithBaseURL(config.RestURL),
		restclient.WithUsername(config.RestUser),
		restclient.WithPassword(config.RestSecret),
		restclient.WithTimeOut(time.Duration(config.RestTimeout)*time.Second),
		restclient.WithDebugRoundTripper())

	if err != nil {
		klog.Errorf("error creating glusterd2 REST client: %s", err.Error())
		return nil
	}

	klog.V(1).Infof("GlusterFS Block CSI driver initialized")

	return gfd
}

// NewControllerServer initialize a controller server for GlusterFS Block CSI driver.
func NewControllerServer(g *GfDriver) *ControllerServer {
	return &ControllerServer{
		GfDriver: g,
	}
}

// NewNodeServer initialize a node server for GlusterFS Block CSI driver.
func NewNodeServer(g *GfDriver) *NodeServer {
	return &NodeServer{
		GfDriver: g,
	}
}

// NewIdentityServer initialize an identity server for GlusterFS Block CSI driver.
func NewIdentityServer(g *GfDriver) *IdentityServer {
	return &IdentityServer{
		GfDriver: g,
	}
}

// Run start a non-blocking grpc controller,node and identityserver for GlusterFS Block CSI driver which can serve multiple parallel requests
func (g *GfDriver) Run() {
	srv := csicommon.NewNonBlockingGRPCServer()
	srv.Start(g.Endpoint, NewIdentityServer(g), NewControllerServer(g), NewNodeServer(g))
	srv.Wait()
}
