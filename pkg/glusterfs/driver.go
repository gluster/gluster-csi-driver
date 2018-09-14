package glusterfs

import (
	"github.com/gluster/gluster-csi-driver/pkg/glusterfs/utils"

	"github.com/gluster/glusterd2/pkg/restclient"
	"github.com/golang/glog"
	"github.com/kubernetes-csi/drivers/pkg/csi-common"
)

//CSI Driver for glusterfs
type GfDriver struct {
	client *restclient.Client
	*utils.Config
	*utils.CsiDrvParam
}

// New returns CSI driver
func New(config *utils.Config, csiConfig *utils.CsiDrvParam) *GfDriver {
	gfd := &GfDriver{}

	if config != nil {
		gfd.Config = config
		gfd.client, _ = restclient.New(config.RestURL, config.RestUser, config.RestSecret, "", false)
	} else {
		glog.Errorf("GlusterFS CSI Driver initialization failed")
		return nil
	}

	gfd.CsiDrvParam = csiConfig
	glog.V(1).Infof("GlusterFS CSI Driver initialized")

	return gfd
}

func NewControllerServer(g *GfDriver) *ControllerServer {
	return &ControllerServer{
		GfDriver: g,
	}
}

func NewNodeServer(g *GfDriver) *NodeServer {
	return &NodeServer{
		GfDriver: g,
	}
}

func NewidentityServer(g *GfDriver) *IdentityServer {
	return &IdentityServer{
		GfDriver: g,
	}
}

func (g *GfDriver) Run() {
	srv := csicommon.NewNonBlockingGRPCServer()
	srv.Start(g.Endpoint, NewidentityServer(g), NewControllerServer(g), NewNodeServer(g))
	srv.Wait()
}
