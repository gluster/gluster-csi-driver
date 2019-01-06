package glusterfs

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/glog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/kubernetes/pkg/util/mount"
	"k8s.io/kubernetes/pkg/volume/util"
)

// NodeServer struct of Glusterfs CSI driver with supported methods of CSI node server spec.
type NodeServer struct {
	*GfDriver
}

var glusterMounter = mount.New("")

// NodeStageVolume mounts the volume to a staging path on the node.
func (ns *NodeServer) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// NodeUnstageVolume unstages the volume from the staging path
func (ns *NodeServer) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// NodePublishVolume mounts the volume mounted to the staging path to the target path
func (ns *NodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	glog.V(2).Infof("received node publish volume request %+v", req)

	if err := ns.validateNodePublishVolumeReq(req); err != nil {
		return nil, err
	}

	targetPath := req.GetTargetPath()

	notMnt, err := glusterMounter.IsLikelyNotMountPoint(targetPath)

	if err != nil {
		if os.IsNotExist(err) {
			if err = os.MkdirAll(targetPath, 0750); err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
			notMnt = true
		} else {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	if !notMnt {
		return &csi.NodePublishVolumeResponse{}, nil
	}

	mo := req.GetVolumeCapability().GetMount().GetMountFlags()

	if req.GetReadonly() {
		mo = append(mo, "ro")
	}
	gs := req.GetVolumeContext()["glusterserver"]

	ep := req.GetVolumeContext()["glustervol"]
	source := fmt.Sprintf("%s:%s", gs, ep)

	err = glusterMounter.Mount(source, targetPath, "glusterfs", mo)
	if err != nil {
		if os.IsPermission(err) {
			return nil, status.Error(codes.PermissionDenied, err.Error())
		}
		if strings.Contains(err.Error(), "invalid argument") {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

// NodeGetVolumeStats returns volume capacity statistics available for the volume
func (ns *NodeServer) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {

	//TODO need to implement volume status call
	return nil, status.Error(codes.Unimplemented, "")

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
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "request cannot be empty")
	}

	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "NodeUnpublishVolume Volume ID must be provided")
	}

	if req.TargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "NodeUnpublishVolume Target Path must be provided")
	}

	targetPath := req.GetTargetPath()
	notMnt, err := glusterMounter.IsLikelyNotMountPoint(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Error(codes.NotFound, "targetpath not found")
		}
		return nil, status.Error(codes.Internal, err.Error())

	}

	if notMnt {
		return nil, status.Error(codes.NotFound, "volume not mounted")
	}

	err = util.UnmountPath(req.GetTargetPath(), glusterMounter)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

// NodeGetInfo returns NodeGetInfoResponse for CO.
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
