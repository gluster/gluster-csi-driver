package glustervirtblock

import (
	"context"
	"os"
	"path"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/gluster/gluster-csi-driver/pkg/gluster-virtblock/config"
	"github.com/gluster/gluster-csi-driver/pkg/utils"
	"github.com/golang/glog"
	"github.com/kubernetes-csi/csi-lib-utils/protosanitizer"
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
func (ns *NodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	glog.V(2).Infof("received node publish volume request %+v", protosanitizer.StripSecrets(req))

	volID := req.GetVolumeId()
	gs := req.GetVolumeContext()["glusterserver"]
	volume := req.GetVolumeContext()["glustervol"]
	mo := req.GetVolumeCapability().GetMount().GetMountFlags()
	if req.GetReadonly() {
		mo = append(mo, ",ro")
	}
	fsType := req.GetVolumeCapability().GetMount().GetFsType()
	targetPath := req.GetTargetPath()

	err := ns.validateNodePublishVolumeReq(req)
	if err != nil {
		return nil, err
	}

	mntInfo, found := ns.Config.Mounts[volume]
	if !found {
		source := gs + ":" + volume
		mntInfo := config.MntInfo{}
		mntInfo.MntPath = path.Join(ns.Config.MntPathPrefix, volume)
		err = utils.MountVolume(source, mntInfo.MntPath, "glusterfs", nil)
		if err != nil {
			return nil, err
		}
		mntInfo.RefCount = 0
		ns.Config.Mounts[volume] = &mntInfo
	}
	srcPath := path.Join(mntInfo.MntPath, volID)

	if _, err = os.Stat(srcPath); os.IsNotExist(err) {
		glog.Errorf("Block volume doesn't exist: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	//TODO check if its raw device req.GetVolumeCapability().GetBlock()
	err = utils.MountVolume(srcPath, targetPath, fsType, mo)
	if err != nil {
		return nil, err
	}

	ns.Config.Mounts[volume].RefCount++

	return &csi.NodePublishVolumeResponse{}, nil
}

// NodeGetVolumeStats returns volume capacity statistics available for the
// volume
func (ns *NodeServer) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {

	// TODO need to implement volume status call
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
	glog.V(2).Infof("received node unpublish volume request %+v", protosanitizer.StripSecrets(req))

	if err := ns.validateNodeUnpublishVolumeReq(req); err != nil {
		return nil, err
	}

	targetPath := req.GetTargetPath()
	volID := req.GetVolumeId()
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

	/* TODO: Or could also extract the host vol from the mount information,
	to avoid a BlockVolumeGet network call
	devicePath, cnt, err := mounter.GetDeviceNameFromMount(mounter, targetPath)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	blkVol := losetup -l devicePath | awk '{ if(NR==2) print $6 }'
	strings.TrimPrefix(blkVol, ns.Config.MntPathPrefix)
	strings.TrimPrefix(blkVol, "/")
	strings.split(blkVol, "/")
	hostVol := blkVol[0]
	*/

	blkVol, err := ns.client.BlockVolumeGet(virtBlockProvider, volID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	ns.Config.Mounts[blkVol.HostingVolume].RefCount--
	if ns.Config.Mounts[blkVol.HostingVolume].RefCount == 0 {
		err = util.UnmountPath(ns.Config.Mounts[blkVol.HostingVolume].MntPath, mounter)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	err = util.UnmountPath(targetPath, mounter)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	err = os.RemoveAll(targetPath)
	if err != nil {
		glog.V(2).Infof("failed to remove target path: %s, err: %v", targetPath, err)
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

// NodeGetInfo info
func (ns *NodeServer) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{
		NodeId: ns.GfDriver.NodeID,
	}, nil
}

// NodeGetCapabilities returns the supported capabilities of the node server
func (ns *NodeServer) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{}, nil
}
