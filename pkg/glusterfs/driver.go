package glusterfs

import (
	"github.com/gluster/gluster-csi-driver/pkg/glusterfs/utils"

	"github.com/gluster/glusterd2/pkg/restclient"
	"github.com/golang/glog"
	"github.com/kubernetes-csi/drivers/pkg/csi-common"
)

const (
	glusterfsCSIDriverName    = "org.gluster.glusterfs"
	glusterfsCSIDriverVersion = "0.0.8"
)

// GfDriver is the struct embedding information about the connection to gluster cluster and configuration of CSI driver.
type GfDriver struct {
	client *restclient.Client
	*utils.Config
}

// New returns CSI driver
func New(config *utils.Config) *GfDriver {
	gfd := &GfDriver{}

	if config != nil {
		gfd.Config = config
		gfd.client, _ = restclient.New(config.RestURL, config.RestUser, config.RestSecret, "", false)
	} else {
		glog.Errorf("GlusterFS CSI Driver initialization failed")
		return nil
	}

	glog.V(1).Infof("GlusterFS CSI Driver initialized")

	return gfd
}

// NewControllerServer initialize a controller server for glusterfs CSI driver.
func NewControllerServer(g *GfDriver) *ControllerServer {
	return &ControllerServer{
		GfDriver: g,
	}
}

// NewNodeServer initialize a node server for glusterfs CSI driver.
func NewNodeServer(g *GfDriver) *NodeServer {
	return &NodeServer{
		GfDriver: g,
	}
}

// NewidentityServer initialize an identity server for glusterfs CSI driver.
func NewidentityServer(g *GfDriver) *IdentityServer {
	return &IdentityServer{
		GfDriver: g,
	}
}

// Run start a non-blocking grpc controller,node and identityserver for glusterfs CSI driver which can serve multiple parallel requests
func (g *GfDriver) Run() {
	srv := csicommon.NewNonBlockingGRPCServer()
	srv.Start(g.Endpoint, NewidentityServer(g), NewControllerServer(g), NewNodeServer(g))
	srv.Wait()
}
