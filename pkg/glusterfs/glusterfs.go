package glusterfs

import (
	"runtime"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"k8s.io/klog/v2"
	"k8s.io/mount-utils"
)

const (
	glusterfsCSIDriverName    = "org.gluster.glusterfs"
	glusterfsCSIDriverVersion = "2.0.0"

	pvcNameKey           = "csi.storage.k8s.io/pvc/name"
	pvcNamespaceKey      = "csi.storage.k8s.io/pvc/namespace"
	pvNameKey            = "csi.storage.k8s.io/pv/name"
	pvcNameMetadata      = "${pvc.metadata.name}"
	pvcNamespaceMetadata = "${pvc.metadata.namespace}"
	pvNameMetadata       = "${pv.metadata.name}"
)

type CSIDriver interface {
	csi.ControllerServer
	csi.NodeServer
	csi.IdentityServer

	Run(endpoint, kubeconfig string, testMode bool)
}

// Driver is the struct embedding information about the connection to gluster
// cluster and configuration of CSI driver.
type Driver struct {
	endpoint string
	name     string
	nodeID   string
	version  string

	ns    *NodeServer
	nscap []*csi.NodeServiceCapability
}

type DriverOptions struct {
	Endpoint       string
	NodeID         string
	DriverName     string
	Kubeconfig     string
	MetricsAddress string
}

// New returns CSI driver
func NewDriver(options *DriverOptions) *Driver {
	klog.V(2).Infof("Driver: %#v version: %v", options, driverVersion)

	if options == nil {
		klog.Errorf("GlusterFS CSI driver initialization failed")
		return nil
	}

	gfd := &Driver{
		endpoint: options.Endpoint,
		name:     options.DriverName,
		version:  driverVersion,
		nodeID:   options.NodeID,
	}

	gfd.AddNodeServiceCapabilities([]csi.NodeServiceCapability_RPC_Type{
		csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
		csi.NodeServiceCapability_RPC_SINGLE_NODE_MULTI_WRITER,
		csi.NodeServiceCapability_RPC_UNKNOWN,
	})

	klog.V(1).Infof("GlusterFS CSI driver initialized")

	return gfd
}

// NewNodeServer initialize a node server for GlusterFS CSI driver.
func NewNodeServer(g *Driver, mounter mount.Interface) *NodeServer {
	return &NodeServer{
		Driver:  g,
		mounter: mounter,
	}
}

// Run start a non-blocking grpc controller,node and identityserver for
// GlusterFS CSI driver which can serve multiple parallel requests
func (g *Driver) Run(endpoint, kubeconfig string, testMode bool) {
	versionMeta, err := GetVersionYAML(g.name)
	if err != nil {
		klog.Fatalf("%v", err)
	}
	klog.V(2).Infof("\nDRIVER INFORMATION:\n-------------------\n%s\n", versionMeta)

	mounter := mount.New("")
	if runtime.GOOS == "linux" {
		// MounterForceUnmounter is only implemented on Linux now
		mounter = mounter.(mount.MounterForceUnmounter)
	}
	g.ns = NewNodeServer(g, mounter)
	srv := NewNonBlockingGRPCServer()
	srv.Start(g.endpoint, NewIdentityServer(g), nil, g.ns, testMode)
	srv.Wait()
}

func (n *Driver) AddNodeServiceCapabilities(nl []csi.NodeServiceCapability_RPC_Type) {
	var nsc []*csi.NodeServiceCapability
	for _, n := range nl {
		nsc = append(nsc, NewNodeServiceCapability(n))
	}
	n.nscap = nsc
}

// replaceWithMap replace key with value for str
func replaceWithMap(str string, m map[string]string) string {
	for k, v := range m {
		if k != "" {
			str = strings.ReplaceAll(str, k, v)
		}
	}
	return str
}

func IsCorruptedDir(dir string) bool {
	_, pathErr := mount.PathExists(dir)
	return pathErr != nil && mount.IsCorruptedMnt(pathErr)
}
