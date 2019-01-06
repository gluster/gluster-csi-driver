package glusterfs

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gluster/gluster-csi-driver/pkg/glusterfs/utils"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/gluster/glusterd2/pkg/api"
	gd2Error "github.com/gluster/glusterd2/pkg/errors"
	"github.com/golang/glog"
	"github.com/golang/protobuf/ptypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	volumeOwnerAnn            = "VolumeOwner"
	defaultVolumeSize   int64 = 1000 * utils.MB // default volume size ie 1 GB
	defaultReplicaCount       = 3
	minReplicaCount           = 1
	maxReplicaCount           = 10
)

var errVolumeNotFound = errors.New("volume not found")

// ControllerServer struct of GlusterFS CSI driver with supported methods of CSI
// controller server spec.
type ControllerServer struct {
	*GfDriver
}

// CsiDrvParam stores csi driver specific request parameters.
// This struct will be used to gather specific fields of CSI driver:
// For eg. csiDrvName, csiDrvVersion..etc and also gather
// parameters passed from SC which not part of gluster volcreate api.
// GlusterCluster - The resturl of gluster cluster
// GlusterUser - The gluster username who got access to the APIs.
// GlusterUserToken - The password/token of glusterUser to connect to
// glusterCluster.
// GlusterVersion - Says the version of the glustercluster
// running in glusterCluster endpoint.
// All of these fields are optional and can be used if needed.
type CsiDrvParam struct {
	GlusterCluster   string
	GlusterUser      string
	GlusterUserToken string
	GlusterVersion   string
	CsiDrvName       string
	CsiDrvVersion    string
}

// ProvisionerConfig is the combined configuration of gluster cli vol create
// request and CSI driver specific input
type ProvisionerConfig struct {
	gdVolReq *api.VolCreateReq
	// csiConf  *CsiDrvParam
}

// ParseCreateVolRequest parse incoming volume create request and fill
// ProvisionerConfig.
func (cs *ControllerServer) ParseCreateVolRequest(req *csi.CreateVolumeRequest) (*ProvisionerConfig, error) {

	var reqConf ProvisionerConfig
	var gdReq api.VolCreateReq
	var err error
	reqConf.gdVolReq = &gdReq

	replicaCount := defaultReplicaCount

	// Get Volume name
	if req != nil {
		reqConf.gdVolReq.Name = req.Name
	}

	for k, v := range req.GetParameters() {

		switch strings.ToLower(k) {

		case "replicas":
			replicas := v
			replicaCount, err = parseVolumeParamInt(replicas)
			if err != nil {
				return nil, fmt.Errorf("invalid value for parameter '%s', %v", k, err)
			}

		default:
			return nil, fmt.Errorf("invalid option %s given for %s CSI driver", k, glusterfsCSIDriverName)
		}
	}

	gdReq.ReplicaCount = replicaCount

	return &reqConf, nil
}

func parseVolumeParamInt(valueString string) (int, error) {

	count, err := strconv.Atoi(valueString)
	if err != nil {
		return 0, fmt.Errorf("value '%s' must be an integer between %d and %d", valueString, minReplicaCount, maxReplicaCount)
	}

	if count < minReplicaCount {
		return 0, fmt.Errorf("value '%s' must be >= %v", valueString, minReplicaCount)
	}
	if count > maxReplicaCount {
		return 0, fmt.Errorf("value '%s' must be <= %v", valueString, maxReplicaCount)
	}

	return count, nil
}

// CreateVolume creates and starts the volume
func (cs *ControllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
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

	err = cs.checkExistingVolume(volumeName, volSizeBytes)
	if err != nil {
		if err != errVolumeNotFound {
			glog.Errorf("error checking for pre-existing volume: %v", err)
			return nil, err
		}

		if req.VolumeContentSource.GetSnapshot().GetSnapshotId() != "" {
			snapName := req.VolumeContentSource.GetSnapshot().GetSnapshotId()
			glog.V(2).Infof("creating volume from snapshot %s", snapName)
			err = cs.checkExistingSnapshot(snapName, req.GetName())
			if err != nil {
				return nil, err
			}
		} else {
			// If volume does not exist, provision volume
			err = cs.doVolumeCreate(volumeName, volSizeBytes)
			if err != nil {
				return nil, err
			}
		}
	}
	err = cs.client.VolumeStart(volumeName, true)
	if err != nil {
		// we dont need to delete the volume if volume start fails as we are
		// listing the volumes and starting it again before sending back the
		// response
		glog.Errorf("failed to start volume: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to start volume: %v", err)
	}

	glusterServer, bkpServers, err := cs.getClusterNodes()
	if err != nil {
		glog.Errorf("failed to get cluster nodes: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to get cluster nodes: %v", err)
	}

	resp := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      volumeName,
			CapacityBytes: volSizeBytes,
			VolumeContext: map[string]string{
				"glustervol":        volumeName,
				"glusterserver":     glusterServer,
				"glusterbkpservers": strings.Join(bkpServers, ":"),
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

func (cs *ControllerServer) checkExistingSnapshot(snapName, volName string) error {
	snapInfo, err := cs.client.SnapshotInfo(snapName)
	if err != nil {
		errResp := cs.client.LastErrorResponse()
		// errResp will be nil in case of No route to host error
		if errResp != nil && errResp.StatusCode == http.StatusNotFound {
			return status.Errorf(codes.NotFound, "failed to get snapshot info %s", err.Error())
		}
		return status.Error(codes.Internal, err.Error())
	}

	if snapInfo.VolInfo.State != api.VolStarted {
		actReq := api.SnapActivateReq{
			Force: true,
		}
		err = cs.client.SnapshotActivate(actReq, snapName)
		if err != nil {
			glog.Errorf("failed to activate snapshot: %v", err)
			return status.Errorf(codes.Internal, "failed to activate snapshot %s", err.Error())
		}
	}
	// create snapshot clone
	err = cs.createSnapshotClone(snapName, volName)
	return err
}

func (cs *ControllerServer) createSnapshotClone(snapName, volName string) error {
	var snapreq api.SnapCloneReq
	snapreq.CloneName = volName
	snapResp, err := cs.client.SnapshotClone(snapName, snapreq)
	if err != nil {
		glog.Errorf("failed to create volume clone: %v", err)
		return status.Errorf(codes.Internal, "failed to create volume clone: %s", err.Error())
	}
	glog.V(1).Infof("snapshot clone response : %+v", snapResp)
	return nil
}

func (cs *ControllerServer) validateCreateVolumeReq(req *csi.CreateVolumeRequest) error {
	if req == nil {
		return status.Errorf(codes.InvalidArgument, "request cannot be empty")
	}

	if req.GetName() == "" {
		return status.Error(codes.InvalidArgument, "name is a required field")
	}

	reqCaps := req.GetVolumeCapabilities()
	if reqCaps == nil {
		return status.Error(codes.InvalidArgument, "volume capabilities is a required field")
	}

	for _, cap := range reqCaps {
		if cap.GetBlock() != nil {
			return status.Error(codes.Unimplemented, "block volume not supported")
		}
	}
	return nil
}

func (cs *ControllerServer) doVolumeCreate(volumeName string, volSizeBytes int64) error {
	glog.V(4).Infof("received request to create volume %s with size %d", volumeName, volSizeBytes)
	volMetaMap := make(map[string]string)
	volMetaMap[volumeOwnerAnn] = glusterfsCSIDriverName
	volumeReq := api.VolCreateReq{
		Name:         volumeName,
		Metadata:     volMetaMap,
		ReplicaCount: defaultReplicaCount,
		Size:         uint64(volSizeBytes),
	}

	glog.V(2).Infof("volume create request: %+v", volumeReq)
	volumeCreateResp, err := cs.client.VolumeCreate(volumeReq)
	if err != nil {
		glog.Errorf("failed to create volume: %v", err)
		errResp := cs.client.LastErrorResponse()
		// errResp will be nil in case of `No route to host` error
		if errResp != nil && errResp.StatusCode == http.StatusConflict {
			return status.Errorf(codes.Aborted, "volume create already in process: %v", err)
		}

		return status.Errorf(codes.Internal, "failed to create volume: %v", err)
	}

	glog.V(3).Infof("volume create response : %+v", volumeCreateResp)
	return nil
}

func (cs *ControllerServer) checkExistingVolume(volumeName string, volSizeBytes int64) error {
	vol, err := cs.client.Volumes(volumeName)
	if err != nil {
		errResp := cs.client.LastErrorResponse()
		// errResp will be nil in case of `No route to host` error
		if errResp != nil && errResp.StatusCode == http.StatusNotFound {
			return errVolumeNotFound
		}
		glog.Errorf("failed to fetch volume : %v", err)
		return status.Errorf(codes.Internal, "error in fetching volume details %v", err)
	}

	volInfo := vol[0]
	// Do the owner validation
	if glusterAnnVal, found := volInfo.Metadata[volumeOwnerAnn]; !found || (found && glusterAnnVal != glusterfsCSIDriverName) {
		return status.Errorf(codes.Internal, "volume %s (%s) is not owned by GlusterFS CSI driver",
			volInfo.Name, volInfo.Metadata)
	}

	if int64(volInfo.Capacity) != volSizeBytes {
		return status.Errorf(codes.AlreadyExists, "volume %s already exits with different size: %d", volInfo.Name, volInfo.Capacity)
	}

	// volume has not started, start the volume
	if volInfo.State != api.VolStarted {
		err = cs.client.VolumeStart(volInfo.Name, true)
		if err != nil {
			return status.Errorf(codes.Internal, "failed to start volume %s: %v", volInfo.Name, err)
		}
	}

	glog.Infof("requested volume %s already exists in the gluster cluster", volumeName)

	return nil
}

func (cs *ControllerServer) getClusterNodes() (string, []string, error) {
	peers, err := cs.client.Peers()
	if err != nil {
		return "", nil, err
	}
	glusterServer := ""
	bkpservers := []string{}

	for i, p := range peers {
		if i == 0 {
			for _, a := range p.PeerAddresses {
				ip := strings.Split(a, ":")
				glusterServer = ip[0]
			}

			continue
		}
		for _, a := range p.PeerAddresses {
			ip := strings.Split(a, ":")
			bkpservers = append(bkpservers, ip[0])
		}

	}
	glog.V(2).Infof("primary and backup gluster servers [%+v,%+v]", glusterServer, bkpservers)

	return glusterServer, bkpservers, err
}

// DeleteVolume deletes the given volume.
func (cs *ControllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "volume delete request is nil")
	}

	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "volume ID is nil")
	}
	glog.V(2).Infof("deleting volume with ID: %s", volumeID)

	// Stop volume
	err := cs.client.VolumeStop(req.VolumeId)

	if err != nil {
		errResp := cs.client.LastErrorResponse()
		// errResp will be nil in case of `No route to host` error
		if errResp != nil && errResp.StatusCode == http.StatusNotFound {
			return &csi.DeleteVolumeResponse{}, nil
		}
		if err.Error() != gd2Error.ErrVolAlreadyStopped.Error() {
			glog.Errorf("failed to stop volume %s: %v", volumeID, err)
			return nil, status.Errorf(codes.Internal, "failed to stop volume %s: %v", volumeID, err)
		}
	}

	// Delete volume
	err = cs.client.VolumeDelete(req.VolumeId)
	if err != nil {
		errResp := cs.client.LastErrorResponse()
		// errResp will be nil in case of No route to host error
		if errResp != nil && errResp.StatusCode == http.StatusNotFound {
			return &csi.DeleteVolumeResponse{}, nil
		}
		glog.Errorf("deleting volume %s failed: %v", req.VolumeId, err)
		return nil, status.Errorf(codes.Internal, "deleting volume %s failed: %v", req.VolumeId, err)
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

// ValidateVolumeCapabilities checks whether the volume capabilities requested
// are supported.
func (cs *ControllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "ValidateVolumeCapabilities() - request is nil")
	}

	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "ValidateVolumeCapabilities() - VolumeId is nil")
	}

	reqCaps := req.GetVolumeCapabilities()
	if reqCaps == nil {
		return nil, status.Error(codes.InvalidArgument, "ValidateVolumeCapabilities() - VolumeCapabilities is nil")
	}

	_, err := cs.client.Volumes(volumeID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "ValidateVolumeCapabilities() - %v", err)
	}

	var vcaps []*csi.VolumeCapability_AccessMode
	for _, mode := range []csi.VolumeCapability_AccessMode_Mode{
		csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY,
		csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY,
		csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
	} {
		vcaps = append(vcaps, &csi.VolumeCapability_AccessMode{Mode: mode})
	}
	capSupport := false

	for _, cap := range reqCaps {
		for _, m := range vcaps {
			if m.Mode == cap.AccessMode.Mode {
				capSupport = true
			}
		}
	}

	if !capSupport {
		return nil, status.Errorf(codes.NotFound, "%v not supported", req.GetVolumeCapabilities())
	}

	resp := &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeCapabilities: req.VolumeCapabilities,
		},
	}

	glog.V(1).Infof("GlusterFS CSI driver volume capabilities: %+v", resp)
	return resp, nil
}

// ListVolumes returns a list of volumes
func (cs *ControllerServer) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	// Fetch all the volumes in the TSP
	volumes, err := cs.client.Volumes("")
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	var entries []*csi.ListVolumesResponse_Entry
	for _, vol := range volumes {
		entries = append(entries, &csi.ListVolumesResponse_Entry{
			Volume: &csi.Volume{
				VolumeId:      vol.Name,
				CapacityBytes: int64(vol.Capacity),
			},
		})
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
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT,
		csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS,
	} {
		caps = append(caps, newCap(cap))
	}

	resp := &csi.ControllerGetCapabilitiesResponse{
		Capabilities: caps,
	}

	return resp, nil
}

// CreateSnapshot create snapshot of an existing PV
func (cs *ControllerServer) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {

	if err := cs.validateCreateSnapshotReq(req); err != nil {
		return nil, err
	}
	glog.V(2).Infof("received request to create snapshot %v from volume %v", req.GetName(), req.GetSourceVolumeId())

	snapInfo, err := cs.client.SnapshotInfo(req.GetName())
	if err != nil {
		glog.Errorf("failed to get snapshot info for %v with Error %v", req.GetName(), err.Error())
		errResp := cs.client.LastErrorResponse()
		// errResp will be nil in case of No route to host error
		if errResp != nil && errResp.StatusCode != http.StatusNotFound {

			return nil, status.Errorf(codes.Internal, "CreateSnapshot - failed to get snapshot info %s", err.Error())
		}
		if errResp == nil {
			return nil, status.Error(codes.Internal, err.Error())
		}

	} else {

		if snapInfo.ParentVolName != req.GetSourceVolumeId() {
			glog.Errorf("snapshot %v belongs to different volume %v", req.GetName(), snapInfo.ParentVolName)
			return nil, status.Errorf(codes.AlreadyExists, "CreateSnapshot - snapshot %s belongs to different volume %s", snapInfo.ParentVolName, req.GetSourceVolumeId())
		}
		createdAt, errT := ptypes.TimestampProto(snapInfo.CreatedAt)
		if errT != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		return &csi.CreateSnapshotResponse{
			Snapshot: &csi.Snapshot{
				SnapshotId:     snapInfo.VolInfo.Name,
				SourceVolumeId: snapInfo.ParentVolName,
				CreationTime:   createdAt,
				SizeBytes:      int64(snapInfo.VolInfo.Capacity),
				ReadyToUse:     true,
			},
		}, nil
	}

	snapResp, err := cs.doSnapshot(req.GetName(), req.GetSourceVolumeId())
	if err != nil {
		return nil, err
	}
	createdAt, err := ptypes.TimestampProto(snapResp.CreatedAt)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &csi.CreateSnapshotResponse{
		Snapshot: &csi.Snapshot{
			SnapshotId:     snapResp.VolInfo.Name,
			SourceVolumeId: snapResp.ParentVolName,
			CreationTime:   createdAt,
			SizeBytes:      int64(snapResp.VolInfo.Capacity),
			ReadyToUse:     true,
		}}, nil
}

func (cs *ControllerServer) doSnapshot(name, sourceVolID string) (*api.SnapCreateResp, error) {
	snapReq := api.SnapCreateReq{
		VolName:  sourceVolID,
		SnapName: name,
		Force:    true,
	}

	glog.V(2).Infof("snapshot request: %+v", snapReq)
	snapResp, err := cs.client.SnapshotCreate(snapReq)
	if err != nil {
		glog.Errorf("failed to create snapshot %v", err)
		errResp := cs.client.LastErrorResponse()
		// errResp will be nil in case of `No route to host` error
		if errResp != nil && errResp.StatusCode == http.StatusConflict {
			return nil, status.Errorf(codes.Aborted, "snapshot create already in process: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "CreateSnapshot - snapshot create failed %s", err.Error())
	}

	actReq := api.SnapActivateReq{
		Force: true,
	}
	err = cs.client.SnapshotActivate(actReq, name)
	if err != nil {
		glog.Errorf("failed to activate snapshot %v", err)
		return nil, status.Errorf(codes.Internal, "failed to activate snapshot %s", err.Error())
	}
	return &snapResp, nil
}

func (cs *ControllerServer) validateCreateSnapshotReq(req *csi.CreateSnapshotRequest) error {
	if req == nil {
		return status.Errorf(codes.InvalidArgument, "CreateSnapshot request is nil")
	}
	if req.GetName() == "" {
		return status.Error(codes.InvalidArgument, "CreateSnapshot - name cannot be nil")
	}

	if req.GetSourceVolumeId() == "" {
		return status.Error(codes.InvalidArgument, "CreateSnapshot - sourceVolumeId is nil")
	}
	if req.GetName() == req.GetSourceVolumeId() {
		// In glusterd2 we cannot create a snapshot as same name as volume name
		return status.Error(codes.InvalidArgument, "CreateSnapshot - sourceVolumeId  and snapshot name cannot be same")
	}
	return nil
}

// DeleteSnapshot delete provided snapshot of a PV
func (cs *ControllerServer) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "DeleteSnapshot request is nil")
	}
	if req.GetSnapshotId() == "" {
		return nil, status.Error(codes.InvalidArgument, "DeleteSnapshot - snapshotId is empty")
	}
	glog.V(4).Infof("deleting snapshot %s", req.GetSnapshotId())

	err := cs.client.SnapshotDeactivate(req.GetSnapshotId())
	if err != nil {
		errResp := cs.client.LastErrorResponse()
		if errResp != nil && errResp.StatusCode == http.StatusNotFound {
			return &csi.DeleteSnapshotResponse{}, nil
		}

		if err.Error() != gd2Error.ErrSnapDeactivated.Error() {
			glog.Errorf("failed to deactivate snapshot %v", err)
			return nil, status.Errorf(codes.Internal, "DeleteSnapshot - failed to deactivate snapshot %s", err.Error())
		}

	}
	err = cs.client.SnapshotDelete(req.SnapshotId)
	if err != nil {
		errResp := cs.client.LastErrorResponse()
		if errResp != nil && errResp.StatusCode == http.StatusNotFound {
			return &csi.DeleteSnapshotResponse{}, nil
		}
		glog.Errorf("failed to delete snapshot %v", err)
		return nil, status.Errorf(codes.Internal, "DeleteSnapshot - failed to delete snapshot %s", err.Error())
	}
	return &csi.DeleteSnapshotResponse{}, nil
}

// ListSnapshots list the snapshots of a PV
func (cs *ControllerServer) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	var (
		snaplist   api.SnapListResp
		err        error
		startToken int32
	)
	if req.GetStartingToken() != "" {
		i, parseErr := strconv.ParseUint(req.StartingToken, 10, 32)
		if parseErr != nil {
			return nil, status.Errorf(codes.Aborted, "invalid starting token %s", parseErr.Error())
		}
		startToken = int32(i)
	}

	if len(req.GetSnapshotId()) != 0 {
		return cs.listSnapshotFromID(req.GetSnapshotId())
	}

	// If volume id is sent
	if len(req.GetSourceVolumeId()) != 0 {
		snaplist, err = cs.client.SnapshotList(req.SourceVolumeId)
		if err != nil {
			errResp := cs.client.LastErrorResponse()
			if errResp != nil && errResp.StatusCode == http.StatusNotFound {
				resp := csi.ListSnapshotsResponse{}
				return &resp, nil
			}
			glog.Errorf("failed to list snapshots %v", err)
			return nil, status.Errorf(codes.Internal, "ListSnapshot - failed to get snapshots %s", err.Error())
		}
	} else {
		// Get all snapshots
		snaplist, err = cs.client.SnapshotList("")
		if err != nil {
			glog.Errorf("failed to list snapshots %v", err)
			return nil, status.Errorf(codes.Internal, "failed to get snapshots %s", err.Error())
		}
	}

	return cs.doPagination(req, snaplist, startToken)
}

func (cs *ControllerServer) listSnapshotFromID(snapID string) (*csi.ListSnapshotsResponse, error) {
	var entries []*csi.ListSnapshotsResponse_Entry
	snap, err := cs.client.SnapshotInfo(snapID)
	if err != nil {
		errResp := cs.client.LastErrorResponse()
		if errResp != nil && errResp.StatusCode == http.StatusNotFound {
			resp := csi.ListSnapshotsResponse{}
			return &resp, nil
		}
		glog.Errorf("failed to get snapshot info %v", err)
		return nil, status.Errorf(codes.NotFound, "ListSnapshot - failed to get snapshot info %s", err.Error())

	}

	createdAt, err := ptypes.TimestampProto(snap.CreatedAt)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	entries = append(entries, &csi.ListSnapshotsResponse_Entry{
		Snapshot: &csi.Snapshot{
			SnapshotId:     snap.VolInfo.Name,
			SourceVolumeId: snap.ParentVolName,
			CreationTime:   createdAt,
			SizeBytes:      int64(snap.VolInfo.Capacity),
			ReadyToUse:     true,
		},
	})

	resp := csi.ListSnapshotsResponse{}
	resp.Entries = entries
	return &resp, nil

}
func (cs *ControllerServer) doPagination(req *csi.ListSnapshotsRequest, snapList api.SnapListResp, startToken int32) (*csi.ListSnapshotsResponse, error) {
	if req.GetStartingToken() != "" && int(startToken) > len(snapList) {
		return nil, status.Error(codes.Aborted, "invalid starting token")
	}

	var entries []*csi.ListSnapshotsResponse_Entry
	for _, snap := range snapList {
		for _, s := range snap.SnapList {
			createdAt, err := ptypes.TimestampProto(s.CreatedAt)
			if err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
			entries = append(entries, &csi.ListSnapshotsResponse_Entry{
				Snapshot: &csi.Snapshot{
					SnapshotId:     s.VolInfo.Name,
					SourceVolumeId: snap.ParentName,
					CreationTime:   createdAt,
					SizeBytes:      int64(s.VolInfo.Capacity),
					ReadyToUse:     true,
				},
			})
		}

	}

	// TODO need to remove paginating code once  glusterd2 issue
	// https://github.com/gluster/glusterd2/issues/372 is merged
	var (
		maximumEntries   = req.MaxEntries
		nextToken        int32
		remainingEntries = int32(len(snapList)) - startToken
		resp             csi.ListSnapshotsResponse
	)

	if maximumEntries == 0 || maximumEntries > remainingEntries {
		maximumEntries = remainingEntries
	}

	resp.Entries = entries[startToken : startToken+maximumEntries]

	if nextToken = startToken + maximumEntries; nextToken < int32(len(snapList)) {
		resp.NextToken = fmt.Sprintf("%d", nextToken)
	}
	return &resp, nil
}
