package glusterblock

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/gluster/gluster-csi-driver/pkg/utils"

	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	"github.com/gluster/glusterd2/pkg/api"
	"github.com/golang/glog"
	"github.com/pborman/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultVolumeSize   int64 = 1000 * utils.MB // default volume size ie 1 GB
	defaultReplicaCount       = 3
)

// ControllerServer struct of GlusterFS Block CSI driver with supported methods of CSI controller server spec.
type ControllerServer struct {
	*GfDriver
}

// CsiDrvParam stores csi driver specific request parameters.
// This struct will be used to gather specific fields of CSI driver:
// For eg. csiDrvName, csiDrvVersion..etc
// and also gather parameters passed from SC which not part of gluster volcreate api.
// GlusterCluster - The resturl of gluster cluster
// GlusterUser - The gluster username who got access to the APIs.
// GlusterUserToken - The password/token of glusterUser to connect to glusterCluster
// GlusterVersion - Says the version of the glustercluster running in glusterCluster endpoint.
// All of these fields are optional and can be used if needed.
type CsiDrvParam struct {
	GlusterCluster   string
	GlusterUser      string
	GlusterUserToken string
	GlusterVersion   string
	CsiDrvName       string
	CsiDrvVersion    string
}

// ProvisionerConfig is the combined configuration of gluster cli vol create request and CSI driver specific input
type ProvisionerConfig struct {
	gdVolReq *api.VolCreateReq
	//csiConf  *CsiDrvParam
}

// ParseCreateVolRequest parse incoming volume create request and fill ProvisionerConfig.
func (cs *ControllerServer) ParseCreateVolRequest(req *csi.CreateVolumeRequest) (*ProvisionerConfig, error) {

	var reqConf ProvisionerConfig
	var gdReq api.VolCreateReq

	reqConf.gdVolReq = &gdReq

	// Get Volume name
	if req != nil {
		reqConf.gdVolReq.Name = req.Name
	}
	return &reqConf, nil
}

// SetupBlockHost mounts all the block volumes
func (cs *ControllerServer) SetupBlockHost() error {
	if cs.Config.Inited {
		return nil
	}

	glusterServer, bkpServers, err := utils.GetClusterNodes(cs.client)
	if err != nil {
		glog.Errorf("failed to get cluster nodes: %v", err)
		return status.Errorf(codes.Internal, "failed to get cluster nodes: %v", err)
	}
	cs.Config.GlusterServer = glusterServer
	cs.Config.BkpServers = bkpServers

	volumes, err := cs.client.Volumes("")
	if err != nil {
		return err
	}

	for _, vol := range volumes {
		source := cs.Config.GlusterServer + ":" + vol.Name
		targetPath := cs.Config.MntPathPrefix + vol.Name
		err = utils.MountVolume(source, targetPath, "glusterfs", nil)
		if err != nil {
			return err
		}
		cs.Config.Mounts[vol.Name] = targetPath
	}

	cs.Config.Inited = true
	return nil
}

// GetBlockHost returns the block host volume that can accomodate the block volume
func (cs *ControllerServer) GetBlockHost(volSizeBytes int64) (string, error) { //nolint: gocyclo
	hostVol := ""
	for vol, mnt := range cs.Config.Mounts {
		statfs := new(syscall.Statfs_t)
		err := syscall.Statfs(mnt, statfs)
		if err != nil {
			return "", err
		}

		freeSize := statfs.Bfree * uint64(statfs.Bsize)
		//TODO:Account for resize of block that could happen
		if freeSize > uint64(volSizeBytes) {
			hostVol = vol
			break
		}
	}
	if hostVol == "" {
		volumeName := uuid.NewRandom().String()
		_, err := cs.client.Volumes(volumeName)
		if err != nil {
			errResp := cs.client.LastErrorResponse()
			if errResp == nil {
				return "", err
			}
			//errResp will be nil in case of `No route to host` error
			if errResp.StatusCode != http.StatusNotFound {
				return "", err
			}
		}
		if err == nil {
			return "", status.Errorf(codes.Internal, "Block host volume creation: uuid conflict", err)
		}

		volumeReq := api.VolCreateReq{
			Name:         volumeName,
			ReplicaCount: defaultReplicaCount,
			Size:         uint64(cs.Config.BlockHostSize),
		}

		_, err = cs.client.VolumeCreate(volumeReq)
		if err != nil {
			return "", err
		}

		err = cs.client.VolumeStart(volumeName, true)
		if err != nil {
			return "", status.Errorf(codes.Internal, "failed to start volume %s: %v", volumeName, err)
		}

		targetPath := cs.Config.MntPathPrefix + volumeName
		source := cs.Config.GlusterServer + ":" + volumeName
		err = utils.MountVolume(source, targetPath, "glusterfs", nil)
		if err != nil {
			return "", err
		}
		cs.Config.Mounts[volumeName] = targetPath
		hostVol = volumeName
	}
	return hostVol, nil
}

// CreateVolume creates and starts the volume
func (cs *ControllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) { //nolint: gocyclo
	glog.V(2).Infof("request received %+v", req)

	if err := cs.validateCreateVolumeReq(req); err != nil {
		return nil, err
	}

	glog.V(1).Infof("creating volume with name %s", req.Name)

	volSizeBytes := cs.getVolumeSize(req)

	// parse the request.
	parseResp, err := cs.ParseCreateVolRequest(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "failed to parse request")
	}

	volumeName := parseResp.gdVolReq.Name

	err = cs.SetupBlockHost()
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to mount block host volumes")
	}

	volume, err := cs.GetBlockHost(volSizeBytes)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get a block host volume")
	}
	hostPath := cs.Config.Mounts[volume]

	if _, err = os.Stat(hostPath); os.IsNotExist(err) {
		glog.Errorf("failed to create block volume as the block hosting path doesn't exist: %v", err)
		return nil, err
	}
	fileName := hostPath + "/" + volumeName
	_, err = os.Create(fileName)
	if err != nil {
		glog.Errorf("failed to create block file: %+v", err)
		return nil, err
	}

	cmd := exec.Command("truncate", fmt.Sprintf("-s %d", volSizeBytes), fileName) //nolint: gosec
	err = cmd.Run()
	if err != nil {
		glog.Errorf("failed to truncate block file: %+v", err)
		return nil, err
	}

	//cmd = exec.Command("losetup", "--show", "--find", fileName) //nolint: gosec
	//device := ""
	//out, err := cmd.Output()
	//if err == nil {
	//	deviceName := strings.Split(string(out), "\n")
	//	device = deviceName[0]
	//}

	cmd = exec.Command("mkfs.xfs", "-f", fileName) //nolint: gosec
	err = cmd.Run()
	if err != nil {
		glog.Errorf("failed to format block file device:%s-- %+v", fileName, err)
		return nil, err
	}

	resp := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			Id:            volumeName,
			CapacityBytes: volSizeBytes,
			Attributes: map[string]string{
				"filename":          fileName,
				"size":              fmt.Sprintf("%v", volSizeBytes),
				"glustervol":        volume,
				"glusterserver":     cs.Config.GlusterServer,
				"glusterbkpservers": strings.Join(cs.Config.BkpServers, ":"),
			},
		},
	}

	glog.V(4).Infof("CSI volume response: %+v", resp)
	return resp, nil
}

func (cs *ControllerServer) getVolumeSize(req *csi.CreateVolumeRequest) int64 {
	// If capacity mentioned, pick that or use default size 1 GB
	volSizeBytes := defaultVolumeSize
	if capRange := req.GetCapacityRange(); capRange != nil {
		volSizeBytes = capRange.GetRequiredBytes()
	}
	return volSizeBytes
}

func (cs *ControllerServer) validateCreateVolumeReq(req *csi.CreateVolumeRequest) error {
	if req == nil {
		return status.Errorf(codes.InvalidArgument, "request cannot be empty")
	}

	if req.GetName() == "" {
		return status.Error(codes.InvalidArgument, "name is a required field")
	}

	if reqCaps := req.GetVolumeCapabilities(); reqCaps == nil {
		return status.Error(codes.InvalidArgument, "volume capabilities is a required field")
	}

	return nil
}

func (cs *ControllerServer) findBlockHost(volumeID string) (string, error) {
	for vol, mntPath := range cs.Config.Mounts {
		dirent, err := ioutil.ReadDir(mntPath)
		if err != nil {
			return "", status.Error(codes.Internal, err.Error())
		}

		for _, blockVol := range dirent {
			if blockVol.Name() == volumeID {
				return vol, nil
			}
		}
	}
	return "", nil
}

// DeleteVolume deletes the given volume.
func (cs *ControllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "volume delete request is nil")
	}

	err := cs.SetupBlockHost()
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to mount block host volumes")
	}

	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "volume ID is nil")
	}
	glog.V(2).Infof("deleting volume with ID: %s", volumeID)

	//TODO: use req.GetVolumeAttributes()["glustervol"] (currently its not implemented)
	volume, err := cs.findBlockHost(volumeID)
	if err != nil {
		return nil, status.Error(codes.Internal, "Couldn't find the block hosting volume")
	}
	if volume == "" {
		return nil, status.Error(codes.Internal, "Block volume not found")
	}

	hostPath := cs.Config.Mounts[volume]
	fileName := hostPath + "/" + volumeID
	err = os.Remove(fileName)
	if err != nil {
		glog.Errorf("deleting volume %s failed: %v", req.VolumeId, err)
		return nil, err
	}

	glog.Infof("successfully deleted volume %s", volumeID)
	return &csi.DeleteVolumeResponse{}, nil
}

// ControllerPublishVolume return Unimplemented error
func (cs *ControllerServer) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ControllerUnpublishVolume return Unimplemented error
func (cs *ControllerServer) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// CreateSnapshot create snapshot of an existing PV
func (cs *ControllerServer) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// DeleteSnapshot delete provided snapshot of a PV
func (cs *ControllerServer) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ListSnapshots list the snapshots of a PV
func (cs *ControllerServer) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ValidateVolumeCapabilities checks whether the volume capabilities requested
// are supported.
func (cs *ControllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) { //nolint: gocyclo
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "ValidateVolumeCapabilities() - request is nil")
	}

	err := cs.SetupBlockHost()
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to mount block host volumes")
	}

	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "ValidateVolumeCapabilities() - VolumeId is nil")
	}

	reqCaps := req.GetVolumeCapabilities()
	if reqCaps == nil {
		return nil, status.Error(codes.InvalidArgument, "ValidateVolumeCapabilities() - VolumeCapabilities is nil")
	}

	var vcaps []*csi.VolumeCapability_AccessMode
	for _, mode := range []csi.VolumeCapability_AccessMode_Mode{
		csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY,
	} {
		vcaps = append(vcaps, &csi.VolumeCapability_AccessMode{Mode: mode})
	}
	capSupport := true
	IsSupport := func(mode csi.VolumeCapability_AccessMode_Mode) bool {
		for _, m := range vcaps {
			if mode == m.Mode {
				return true
			}
		}
		return false
	}
	for _, cap := range reqCaps {
		if !IsSupport(cap.AccessMode.Mode) {
			capSupport = false
		}
	}

	resp := &csi.ValidateVolumeCapabilitiesResponse{
		Supported: capSupport,
	}

	glog.V(1).Infof("GlusterFS Block CSI driver volume capabilities: %+v", resp)
	return resp, nil
}

// ListVolumes returns a list of volumes
func (cs *ControllerServer) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	var entries []*csi.ListVolumesResponse_Entry

	err := cs.SetupBlockHost()
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to mount block host volumes")
	}

	for _, mntPath := range cs.Config.Mounts {
		dirent, err := ioutil.ReadDir(mntPath)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}

		for _, blockVol := range dirent {
			entries = append(entries, &csi.ListVolumesResponse_Entry{
				Volume: &csi.Volume{
					Id:            blockVol.Name(),
					CapacityBytes: blockVol.Size(),
				},
			})
		}
	}
	resp := &csi.ListVolumesResponse{
		Entries: entries,
	}

	return resp, nil
}

// GetCapacity returns the capacity of the storage pool
func (cs *ControllerServer) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ControllerGetCapabilities returns the capabilities of the controller service.
func (cs *ControllerServer) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	newCap := func(cap csi.ControllerServiceCapability_RPC_Type) *csi.ControllerServiceCapability {
		return &csi.ControllerServiceCapability{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: cap,
				},
			},
		}
	}

	var caps []*csi.ControllerServiceCapability
	for _, cap := range []csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		csi.ControllerServiceCapability_RPC_LIST_VOLUMES,
	} {
		caps = append(caps, newCap(cap))
	}

	resp := &csi.ControllerGetCapabilitiesResponse{
		Capabilities: caps,
	}

	return resp, nil
}
