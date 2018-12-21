package node

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gluster/gluster-csi-driver/pkg/command"

	api "github.com/container-storage-interface/spec/lib/go/csi/v0"
	"github.com/golang/glog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/kubernetes/pkg/util/mount"
	"k8s.io/kubernetes/pkg/volume/util"
)

// Server struct of Glusterfs CSI driver with supported methods of CSI node server spec.
type Server struct {
	*command.Config
	Mounter mount.Interface
}

// NewServer instantiates a Server
func NewServer(config *command.Config, mounter mount.Interface) *Server {
	ns := &Server{
		Config:  config,
		Mounter: mounter,
	}

	return ns
}

// NodeStageVolume mounts the volume to a staging path on the node.
func (ns *Server) NodeStageVolume(ctx context.Context, req *api.NodeStageVolumeRequest) (*api.NodeStageVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// NodeUnstageVolume unstages the volume from the staging path
func (ns *Server) NodeUnstageVolume(ctx context.Context, req *api.NodeUnstageVolumeRequest) (*api.NodeUnstageVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// NodePublishVolume mounts the volume mounted to the staging path to the target path
func (ns *Server) NodePublishVolume(ctx context.Context, req *api.NodePublishVolumeRequest) (*api.NodePublishVolumeResponse, error) {
	glog.V(2).Infof("received node publish volume request %+v", req)

	if err := ns.validateNodePublishVolumeReq(req); err != nil {
		return nil, err
	}

	targetPath := req.GetTargetPath()

	notMnt, err := ns.Mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil && os.IsNotExist(err) {
		err = os.MkdirAll(targetPath, 0750)
		notMnt = true
	}
	if err != nil {
		glog.Errorf("error with target path %s: %v", targetPath, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !notMnt {
		return &api.NodePublishVolumeResponse{}, nil
	}

	mo := req.GetVolumeCapability().GetMount().GetMountFlags()

	if req.GetReadonly() {
		mo = append(mo, "ro")
	}

	volAttr := req.GetVolumeAttributes()
	gs := volAttr["glusterserver"]
	ep := volAttr["glustervol"]

	source := fmt.Sprintf("%s:%s", gs, ep)

	if err = ns.mountGlusterVolume(source, targetPath, mo); err != nil {
		return nil, err
	}

	return &api.NodePublishVolumeResponse{}, nil
}

func (ns *Server) mountGlusterVolume(source, targetPath string, mountOptions []string) error {
	err := ns.Mounter.Mount(source, targetPath, "glusterfs", mountOptions)
	if err != nil {
		glog.Errorf("error mounting volume: %v", err)
		code := codes.Internal
		if os.IsPermission(err) {
			code = codes.PermissionDenied
		}
		if strings.Contains(err.Error(), "invalid argument") {
			code = codes.InvalidArgument
		}
		err = status.Error(code, err.Error())
	}

	return err
}

func (ns *Server) validateNodePublishVolumeReq(req *api.NodePublishVolumeRequest) error {
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
func (ns *Server) NodeUnpublishVolume(ctx context.Context, req *api.NodeUnpublishVolumeRequest) (*api.NodeUnpublishVolumeResponse, error) {
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
	notMnt, err := ns.Mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, "target path [%s] not found", targetPath)
		}
		return nil, status.Error(codes.Internal, err.Error())

	}

	if notMnt {
		return nil, status.Error(codes.NotFound, "volume not mounted")
	}

	err = util.UnmountPath(req.GetTargetPath(), ns.Mounter)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &api.NodeUnpublishVolumeResponse{}, nil
}

// NodeGetId returns NodeGetIdResponse for CO.
func (ns *Server) NodeGetId(ctx context.Context, req *api.NodeGetIdRequest) (*api.NodeGetIdResponse, error) {
	return &api.NodeGetIdResponse{
		NodeId: ns.NodeID,
	}, nil
}

// NodeGetInfo info
func (ns *Server) NodeGetInfo(ctx context.Context, req *api.NodeGetInfoRequest) (*api.NodeGetInfoResponse, error) {
	return &api.NodeGetInfoResponse{
		NodeId: ns.NodeID,
	}, nil
}

// NodeGetCapabilities returns the supported capabilities of the node server
func (ns *Server) NodeGetCapabilities(ctx context.Context, req *api.NodeGetCapabilitiesRequest) (*api.NodeGetCapabilitiesResponse, error) {
	// currently there is a single Server capability according to the spec
	nscap := &api.NodeServiceCapability{
		Type: &api.NodeServiceCapability_Rpc{
			Rpc: &api.NodeServiceCapability_RPC{
				Type: api.NodeServiceCapability_RPC_UNKNOWN,
			},
		},
	}
	glog.V(1).Infof("node capabilities: %+v", nscap)
	return &api.NodeGetCapabilitiesResponse{
		Capabilities: []*api.NodeServiceCapability{
			nscap,
		},
	}, nil
}
