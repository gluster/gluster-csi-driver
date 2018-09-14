package controller

import (
	"context"
	"fmt"
	"strings"

	"github.com/gluster/gluster-csi-driver/pkg/client"
	"github.com/gluster/gluster-csi-driver/pkg/command"
	"github.com/gluster/gluster-csi-driver/pkg/identity"

	api "github.com/container-storage-interface/spec/lib/go/csi/v0"
	"github.com/golang/glog"
	csi "github.com/kubernetes-csi/drivers/pkg/csi-common"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server takes a configuration and a GlusterClients cache
type Server struct {
	*command.Config
	client client.GlusterClient
}

// Run starts the controller server
func Run(config *command.Config, cli client.GlusterClient) {
	srv := csi.NewNonBlockingGRPCServer()
	srv.Start(config.Endpoint, identity.NewServer(config), NewServer(config, cli), nil)
	srv.Wait()
}

// NewServer instantiates a Server
func NewServer(config *command.Config, cli client.GlusterClient) *Server {
	cs := &Server{
		Config: config,
		client: cli,
	}

	return cs
}

// CreateVolume creates and starts the volume
func (cs *Server) CreateVolume(ctx context.Context, req *api.CreateVolumeRequest) (*api.CreateVolumeResponse, error) {
	glog.V(2).Infof("request received %+v", req)

	if err := cs.validateCreateVolumeReq(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	volumeName := req.GetName()

	// If capacity mentioned, pick that or use default size 1 GB
	volSizeBytes := client.GB
	if capRange := req.GetCapacityRange(); capRange != nil {
		volSizeBytes = capRange.GetRequiredBytes()
	}

	gc := cs.client

	glog.V(1).Infof("creating volume with name %s", volumeName)

	err := gc.CheckExistingVolume(volumeName, volSizeBytes)
	if err != nil {
		if !client.IsErrNotFound(err) {
			glog.Error(err.Error())
			return nil, status.Error(codes.AlreadyExists, err.Error())
		}

		err = cs.doCreateVolume(gc, req, volSizeBytes)
		if err != nil {
			return nil, err
		}
	} else {
		glog.V(1).Infof("requested volume %s already exists in the gluster cluster", volumeName)
	}

	glusterServers, err := gc.GetClusterNodes(volumeName)
	if err != nil {
		glog.Errorf("failed to get cluster nodes for %s: %v", volumeName, err)
		return nil, status.Errorf(codes.Internal, "failed to get cluster nodes for %s: %v", volumeName, err)
	}
	glog.V(2).Infof("gluster servers: %+v", glusterServers)

	params := map[string]string{}
	params["glustervol"] = volumeName
	params["glusterserver"] = glusterServers[0]
	params["glusterbkpservers"] = strings.Join(glusterServers[1:], ":")

	resp := &api.CreateVolumeResponse{
		Volume: &api.Volume{
			Id:            volumeName,
			CapacityBytes: volSizeBytes,
			Attributes:    params,
		},
	}

	glog.V(4).Infof("CSI CreateVolume response: %+v", resp)
	return resp, nil
}

func (cs *Server) doCreateVolume(gc client.GlusterClient, req *api.CreateVolumeRequest, volSizeBytes int64) error {
	volumeName := req.GetName()
	reqParams := req.GetParameters()
	snapName := req.GetVolumeContentSource().GetSnapshot().GetId()

	if snapName != "" {
		_, err := gc.CheckExistingSnapshot(snapName, volumeName)
		if err != nil {
			glog.Error(err.Error())
			code := codes.Internal
			if client.IsErrExists(err) {
				code = codes.AlreadyExists
			} else if client.IsErrNotFound(err) {
				code = codes.NotFound
			}
			return status.Error(code, err.Error())
		}

		err = gc.CloneSnapshot(snapName, volumeName)
		if err != nil {
			glog.Error(err.Error())
			return status.Error(codes.Internal, err.Error())
		}
	} else {
		// If volume does not exist, provision volume
		glog.V(4).Infof("received request to create volume %s with size %d", volumeName, volSizeBytes)
		err := gc.CreateVolume(volumeName, volSizeBytes, reqParams)
		if err != nil {
			glog.Errorf("failed to create volume: %v", err)
			return status.Errorf(codes.Internal, "failed to create volume: %v", err)
		}
	}

	return nil
}

func (cs *Server) validateCreateVolumeReq(req *api.CreateVolumeRequest) error {
	if req == nil {
		return fmt.Errorf("request cannot be empty")
	}
	if req.GetName() == "" {
		return fmt.Errorf("Name is a required field")
	}
	if reqCaps := req.GetVolumeCapabilities(); reqCaps == nil {
		return fmt.Errorf("VolumeCapabilities is a required field")
	}
	return nil
}

// DeleteVolume deletes the given volume.
func (cs *Server) DeleteVolume(ctx context.Context, req *api.DeleteVolumeRequest) (*api.DeleteVolumeResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "volume delete request is nil")
	}

	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "VolumeId is a required field")
	}
	glog.V(2).Infof("deleting volume with ID: %s", volumeID)

	gc := cs.client
	err := gc.DeleteVolume(volumeID)
	if err != nil && !client.IsErrNotFound(err) {
		glog.Errorf("error deleting volume %s: %v", volumeID, err)
		return nil, status.Errorf(codes.Internal, "error deleting volume %s: %v", volumeID, err)
	} else if client.IsErrNotFound(err) {
		glog.Warningf("volume %s not found", volumeID)
	}

	glog.Infof("successfully deleted volume %s", volumeID)
	return &api.DeleteVolumeResponse{}, nil
}

// ControllerPublishVolume return Unimplemented error
func (cs *Server) ControllerPublishVolume(ctx context.Context, req *api.ControllerPublishVolumeRequest) (*api.ControllerPublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ControllerUnpublishVolume return Unimplemented error
func (cs *Server) ControllerUnpublishVolume(ctx context.Context, req *api.ControllerUnpublishVolumeRequest) (*api.ControllerUnpublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ValidateVolumeCapabilities checks whether the volume capabilities requested
// are supported.
func (cs *Server) ValidateVolumeCapabilities(ctx context.Context, req *api.ValidateVolumeCapabilitiesRequest) (*api.ValidateVolumeCapabilitiesResponse, error) {
	if err := cs.validateVolumeCapabilitiesReq(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	volumeID := req.GetVolumeId()
	reqCaps := req.GetVolumeCapabilities()

	gc := cs.client
	err := gc.CheckExistingVolume(volumeID, 0)
	if err != nil {
		code := codes.Internal
		if client.IsErrNotFound(err) {
			code = codes.NotFound
		}
		glog.Error(err.Error())
		return nil, status.Error(code, err.Error())
	}

	var vcaps []*api.VolumeCapability_AccessMode
	for _, mode := range []api.VolumeCapability_AccessMode_Mode{
		api.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		api.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY,
		api.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY,
		api.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
	} {
		vcaps = append(vcaps, &api.VolumeCapability_AccessMode{Mode: mode})
	}
	capSupport := true
	IsSupport := func(mode api.VolumeCapability_AccessMode_Mode) bool {
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

	resp := &api.ValidateVolumeCapabilitiesResponse{
		Supported: capSupport,
	}
	glog.V(1).Infof("GlusterFS CSI driver volume capabilities: %v", resp)
	return resp, nil
}

func (cs *Server) validateVolumeCapabilitiesReq(req *api.ValidateVolumeCapabilitiesRequest) error {
	if req == nil {
		return fmt.Errorf("request cannot be empty")
	}
	if req.GetVolumeId() == "" {
		return fmt.Errorf("VolumeId is a required field")
	}
	if req.GetVolumeCapabilities() == nil {
		return fmt.Errorf("VolumeCapabilities is a required field")
	}
	return nil
}

// ListVolumes returns a list of volumes
func (cs *Server) ListVolumes(ctx context.Context, req *api.ListVolumesRequest) (*api.ListVolumesResponse, error) {
	start, end, err := listParseRange(req.GetStartingToken(), req.GetMaxEntries())
	if err != nil {
		glog.Error(err)
		return nil, status.Error(codes.Aborted, err.Error())
	}

	entries := []*api.ListVolumesResponse_Entry{}

	gc := cs.client
	vols, err := gc.ListVolumes()
	if err != nil {
		glog.Error(err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	for _, vol := range vols {
		entries = append(entries, &api.ListVolumesResponse_Entry{Volume: vol})
	}

	end, endStr, err := listGetEnd(len(entries), start, end)
	if err != nil {
		glog.Error(err)
		return nil, status.Error(codes.Aborted, err.Error())
	}
	resp := &api.ListVolumesResponse{
		Entries:   entries[start:end],
		NextToken: endStr,
	}
	return resp, nil
}

// GetCapacity returns the capacity of the storage pool
func (cs *Server) GetCapacity(ctx context.Context, req *api.GetCapacityRequest) (*api.GetCapacityResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ControllerGetCapabilities returns the capabilities of the controller service.
func (cs *Server) ControllerGetCapabilities(ctx context.Context, req *api.ControllerGetCapabilitiesRequest) (*api.ControllerGetCapabilitiesResponse, error) {
	newCap := func(cap api.ControllerServiceCapability_RPC_Type) *api.ControllerServiceCapability {
		return &api.ControllerServiceCapability{
			Type: &api.ControllerServiceCapability_Rpc{
				Rpc: &api.ControllerServiceCapability_RPC{
					Type: cap,
				},
			},
		}
	}

	var caps []*api.ControllerServiceCapability
	for _, cap := range []api.ControllerServiceCapability_RPC_Type{
		api.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		api.ControllerServiceCapability_RPC_LIST_VOLUMES,
		api.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT,
		api.ControllerServiceCapability_RPC_LIST_SNAPSHOTS,
	} {
		caps = append(caps, newCap(cap))
	}

	resp := &api.ControllerGetCapabilitiesResponse{
		Capabilities: caps,
	}

	return resp, nil
}

// CreateSnapshot create snapshot of an existing PV
func (cs *Server) CreateSnapshot(ctx context.Context, req *api.CreateSnapshotRequest) (*api.CreateSnapshotResponse, error) {

	if err := cs.validateCreateSnapshotReq(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	snapName := req.GetName()
	srcVol := req.GetSourceVolumeId()

	glog.V(2).Infof("received request to create snapshot %s from volume %s", snapName, srcVol)

	gc := cs.client
	err := gc.CheckExistingVolume(srcVol, 0)
	if err != nil {
		code := codes.Internal
		if client.IsErrNotFound(err) {
			code = codes.AlreadyExists
		}
		glog.Error(err)
		return nil, status.Errorf(code, "error finding volume %s for snapshot %s: %v", srcVol, snapName, err)
	}

	snap, err := gc.CheckExistingSnapshot(snapName, srcVol)
	if err != nil {
		if !client.IsErrNotFound(err) {
			code := codes.Internal
			if client.IsErrExists(err) {
				code = codes.AlreadyExists
			}
			glog.Error(err.Error())
			return nil, status.Error(code, err.Error())
		}

		snap, err = gc.CreateSnapshot(snapName, srcVol)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	return &api.CreateSnapshotResponse{Snapshot: snap}, nil
}

func (cs *Server) validateCreateSnapshotReq(req *api.CreateSnapshotRequest) error {
	if req == nil {
		return fmt.Errorf("request cannot be empty")
	}
	if req.GetName() == "" {
		return fmt.Errorf("Name is a required field")
	}
	if req.GetSourceVolumeId() == "" {
		return fmt.Errorf("SourceVolumeId is a required field")
	}
	if req.GetName() == req.GetSourceVolumeId() {
		return fmt.Errorf("SourceVolumeId and Name cannot be same")
	}
	return nil
}

// DeleteSnapshot delete provided snapshot of a PV
func (cs *Server) DeleteSnapshot(ctx context.Context, req *api.DeleteSnapshotRequest) (*api.DeleteSnapshotResponse, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "DeleteSnapshot request is nil")
	}
	if req.GetSnapshotId() == "" {
		return nil, status.Error(codes.InvalidArgument, "DeleteSnapshot - snapshotId is empty")
	}
	glog.V(4).Infof("deleting snapshot %s", req.GetSnapshotId())

	snapName := req.GetSnapshotId()

	gc := cs.client
	err := gc.DeleteSnapshot(snapName)
	if err != nil && !client.IsErrNotFound(err) {
		glog.Error(err)
		return nil, status.Errorf(codes.Internal, "error deleting snapshot %s: %v", snapName, err)
	} else if client.IsErrNotFound(err) {
		glog.Warningf("snapshot %s not found", snapName)
	}

	glog.Infof("successfully deleted snapshot %s", snapName)
	return &api.DeleteSnapshotResponse{}, nil
}

// ListSnapshots list the snapshots of a PV
func (cs *Server) ListSnapshots(ctx context.Context, req *api.ListSnapshotsRequest) (*api.ListSnapshotsResponse, error) {
	start, end, err := listParseRange(req.GetStartingToken(), req.GetMaxEntries())
	if err != nil {
		glog.Error(err)
		return nil, status.Error(codes.Aborted, err.Error())
	}

	entries := []*api.ListSnapshotsResponse_Entry{}
	snapName := req.GetSnapshotId()
	srcVol := req.GetSourceVolumeId()

	gc := cs.client
	snaps, err := gc.ListSnapshots(snapName, srcVol)
	if err != nil {
		glog.Errorf("failed to list snapshots: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to list snapshots: %v", err)
	}

	for _, snap := range snaps {
		entries = append(entries, &api.ListSnapshotsResponse_Entry{Snapshot: snap})
	}

	end, endStr, err := listGetEnd(len(entries), start, end)
	if err != nil {
		glog.Error(err)
		return nil, status.Error(codes.Aborted, err.Error())
	}
	resp := &api.ListSnapshotsResponse{
		Entries:   entries[start:end],
		NextToken: endStr,
	}
	return resp, err
}
