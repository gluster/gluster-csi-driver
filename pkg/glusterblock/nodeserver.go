package glusterblock

import (
	"context"
	"os"

	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	"github.com/gluster/gluster-csi-driver/pkg/utils"
	"github.com/golang/glog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/kubernetes/pkg/util/mount"
	"k8s.io/kubernetes/pkg/volume/util"
)

// NodeServer struct of GlusterFS Block CSI driver with supported methods of CSI node server spec.
type NodeServer struct {
	*GfDriver
}

var mounter = mount.New("")

// NodeStageVolume mounts the volume to a staging path on the node.
func (ns *NodeServer) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// NodeUnstageVolume unstages the volume from the staging path
func (ns *NodeServer) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// NodePublishVolume mounts the volume mounted to the staging path to the target path
func (ns *NodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) { //nolint: gocyclo
	glog.V(2).Infof("received node publish volume request %+v", req)

	err := ns.validateNodePublishVolumeReq(req)
	if err != nil {
		return nil, err
	}

	gs := req.GetVolumeAttributes()["glusterserver"]
	volume := req.GetVolumeAttributes()["glustervol"]
	hostPath, found := ns.Config.Mounts[volume]
	if !found {
		source := gs + ":" + volume
		hostPath = ns.Config.MntPathPrefix + volume
		err = utils.MountVolume(source, hostPath, "glusterfs", nil)
		if err != nil {
			return nil, err
		}
		ns.Config.Mounts[volume] = hostPath
	}
	srcPath := hostPath + "/" + req.GetVolumeId()

	if _, err = os.Stat(srcPath); os.IsNotExist(err) {
		glog.Errorf("Volume doesn't exist: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	mo := req.GetVolumeCapability().GetMount().GetMountFlags()
	if req.GetReadonly() {
		mo = append(mo, ",ro")
	}
	targetPath := req.GetTargetPath()

	err = utils.MountVolume(srcPath, targetPath, "xfs", mo)
	if err != nil {
		return nil, err
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

func (ns *NodeServer) validateNodePublishVolumeReq(req *csi.NodePublishVolumeRequest) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request cannot be empty")
	}

	if req.GetVolumeId() == "" {
		return status.Error(codes.InvalidArgument, "NodePublishVolume Volume ID must be provided")
	}

	if req.GetTargetPath() == "" {
		return status.Error(codes.InvalidArgument, "NodePublishVolume Target Path cannot be empty")
	}

	if req.GetVolumeCapability() == nil {
		return status.Error(codes.InvalidArgument, "NodePublishVolume Volume Capability must be provided")
	}
	return nil
}

// NodeUnpublishVolume unmounts the volume from the target path
func (ns *NodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	glog.V(2).Infof("received node unpublish volume request %+v", req)

	if err := ns.validateNodeUnpublishVolumeReq(req); err != nil {
		return nil, err
	}

	targetPath := req.GetTargetPath()
	notMnt, err := mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Error(codes.NotFound, "targetpath not found")
		}
		return nil, status.Error(codes.Internal, err.Error())

	}

	if notMnt {
		return nil, status.Error(codes.NotFound, "volume not mounted")
	}

	err = util.UnmountPath(targetPath, mounter)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (ns *NodeServer) validateNodeUnpublishVolumeReq(req *csi.NodeUnpublishVolumeRequest) error {
	if req == nil {
		return status.Errorf(codes.InvalidArgument, "request cannot be empty")
	}

	if req.VolumeId == "" {
		return status.Error(codes.InvalidArgument, "NodeUnpublishVolume Volume ID must be provided")
	}

	if req.TargetPath == "" {
		return status.Error(codes.InvalidArgument, "NodeUnpublishVolume Target Path must be provided")
	}
	return nil
}

// NodeGetId returns NodeGetIdResponse for CO.
func (ns *NodeServer) NodeGetId(ctx context.Context, req *csi.NodeGetIdRequest) (*csi.NodeGetIdResponse, error) {
	return &csi.NodeGetIdResponse{
		NodeId: ns.GfDriver.NodeID,
	}, nil
}

// NodeGetInfo info
func (ns *NodeServer) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{
		NodeId: ns.GfDriver.NodeID,
	}, nil
}

// NodeGetCapabilities returns the supported capabilities of the node server
func (ns *NodeServer) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	// currently there is a single NodeServer capability according to the spec
	nscap := &csi.NodeServiceCapability{
		Type: &csi.NodeServiceCapability_Rpc{
			Rpc: &csi.NodeServiceCapability_RPC{
				Type: csi.NodeServiceCapability_RPC_UNKNOWN,
			},
		},
	}
	glog.V(1).Infof("node capabiilities: %+v", nscap)
	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: []*csi.NodeServiceCapability{
			nscap,
		},
	}, nil
}
