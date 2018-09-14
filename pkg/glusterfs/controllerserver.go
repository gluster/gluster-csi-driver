package glusterfs

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gluster/gluster-csi-driver/pkg/glusterfs/utils"

	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	"github.com/golang/glog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	volumeOwnerAnn            = "VolumeOwner"
	defaultVolumeSize   int64 = 1000 * utils.MB // default volume size ie 1 GB
	defaultReplicaCount       = 3
)

var errVolumeNotFound = errors.New("volume not found")

type ControllerServer struct {
	*GfDriver
}

//CreateVolume creates and starts the volume
func (cs *ControllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	glog.V(2).Infof("Request received %+v", req)

	if err := cs.validateCreateVolumeReq(req); err != nil {
		return nil, err
	}

	glog.V(1).Infof("creating volume with name %s", req.Name)

	if req.VolumeCapabilities == nil || len(req.VolumeCapabilities) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume capabilities is a required field")
	}

	// If capacity mentioned, pick that or use default size 1 GB
	volSizeBytes := defaultVolumeSize
	if capRange := req.GetCapacityRange(); capRange != nil {
		volSizeBytes = capRange.GetRequiredBytes()
	}

	volumeName := req.Name
	glusterVol := req.GetParameters()["glustervol"]
	glusterServer := req.GetParameters()["glusterserver"]
	glusterServerKind := req.GetParameters()["glusterserverkind"]
	glusterURL := req.GetParameters()["glusterurl"]
	glusterUser := req.GetParameters()["glusteruser"]
	glusterUserSecret := req.GetParameters()["glusterusersecret"]

	glog.V(3).Infof("Request fields:[ %v %v %v %v %v]", glusterVol, glusterServer, glusterURL, glusterUser, glusterUserSecret)

	gc := &GlusterClient{
		kind:     GlusterServerKind(glusterServerKind),
		url:      glusterURL,
		username: glusterUser,
		password: glusterUserSecret,
	}
	err := glusterClientCache.Init(gc)
	if err != nil {
		glog.Error(err)
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}

	err = gc.CheckExistingVolume(volumeName, volSizeBytes)
	if err != nil && err != errVolumeNotFound {
		glog.Errorf("Error with pre-existing volume: %v", err)
		return nil, status.Errorf(codes.Internal, "Error with pre-existing volume: %v", err)
	} else if err == errVolumeNotFound {
		// If volume does not exist, provision volume
		glog.V(4).Infof("Received request to create volume %s with size %d", volumeName, volSizeBytes)
		err = gc.CreateVolume(volumeName, volSizeBytes)
		if err != nil {
			glog.Errorf("failed to create volume: %v", err)
			return nil, status.Errorf(codes.Internal, "failed to create volume: %v", err)
		}
	}

	glusterServers, err := gc.GetClusterNodes(volumeName)
	if err != nil {
		glog.Errorf("Failed to get cluster nodes for %s: %v", volumeName, err)
		return nil, status.Errorf(codes.Internal, "Failed to get cluster nodes for %s: %v", volumeName, err)
	}
	glusterServer = glusterServers[0]
	bkpservers = glusterServers[1:]

	resp := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			Id:            volumeName,
			CapacityBytes: volSizeBytes,
			Attributes: map[string]string{
				"glustervol":           volumeName,
				"glusterserver":        glusterServer,
				"glusterbkpservers":    strings.Join(bkpServers, ":"),
				"glusterserververkind": string(gc.kind),
				"glusterurl":           glusterURL,
				"glusteruser":          glusterUser,
				"glusterusersecret":    glusterUserSecret,
			},
		},
	}

	glog.V(4).Infof("CSI Volume response: %+v", resp)
	return resp, nil
}

func (cs *ControllerServer) validateCreateVolumeReq(req *csi.CreateVolumeRequest) error {
	if req == nil {
		return status.Errorf(codes.InvalidArgument, "request cannot be empty")
	}

	if req.GetName() == "" {
		return status.Error(codes.InvalidArgument, "Name is a required field")
	}

	if req.GetVolumeCapabilities() == nil || len(req.GetVolumeCapabilities()) == 0 {
		return status.Error(codes.InvalidArgument, "Volume capabilities is a required field")
	}

	return nil
}

// DeleteVolume deletes the given volume.
func (cs *ControllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume delete request is nil")
	}

	volumeId := req.GetVolumeId()
	if volumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "volume ID is nil")
	}
	glog.V(2).Infof("Deleting volume with ID: %s", volumeId)

	gc, err := glusterClientCache.FindVolumeClient(volumeId)
	if err != nil && err != errVolumeNotFound {
		glog.Error(err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("error deleting volume %s: %s", volumeId, err.Error()))
	} else if err == nil {
		err = gc.DeleteVolume(volumeId)
		if err != nil && err != errVolumeNotFound {
			glog.Errorf("error deleting volume %s: %v", volumeId, err)
			return nil, status.Errorf(codes.Internal, "error deleting volume %s: %v", volumeId, err)
		}
	} else {
		glog.Warningf("Volume %s not found: %s", volumeId, err)
	}

	glog.Infof("Successfully deleted volume %s", volumeId)
	return &csi.DeleteVolumeResponse{}, nil
}

// ControllerPublishVolume return Unimplemented error
func (cs *ControllerServer) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

//ControllerUnpublishVolume return Unimplemented error
func (cs *ControllerServer) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ValidateVolumeCapabilities checks whether the volume capabilities requested
// are supported.
func (cs *ControllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "ValidateVolumeCapabilities() - request is nil")
	}

	volumeId := req.GetVolumeId()
	if volumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "ValidateVolumeCapabilities() - VolumeId is nil")
	}

	if req.VolumeCapabilities == nil {
		return nil, status.Error(codes.InvalidArgument, "ValidateVolumeCapabilities() - VolumeCapabilities is nil")
	}

	_, err := glusterClientCache.FindVolumeClient(volumeId)
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
	capSupport := true
	IsSupport := func(mode csi.VolumeCapability_AccessMode_Mode) bool {
		for _, m := range vcaps {
			if mode == m.Mode {
				return true
			}
		}
		return false
	}
	for _, cap := range req.VolumeCapabilities {
		if !IsSupport(cap.AccessMode.Mode) {
			capSupport = false
		}
	}

	resp := &csi.ValidateVolumeCapabilitiesResponse{
		Supported: capSupport,
	}
	glog.V(1).Infof("GlusterFS CSI driver volume capabilities: %v", resp)
	return resp, nil
}

// ListVolumes returns a list of volumes
func (cs *ControllerServer) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	entries := []*csi.ListVolumesResponse_Entry{}

	start, err := strconv.Atoi(req.StartingToken)
	if err != nil {
		glog.Errorf("Invalid StartingToken: %s", req.StartingToken)
		return nil, status.Errorf(codes.InvalidArgument, "Invalid StartingToken: %s", req.StartingToken)
	}
	end := int32(start) + req.MaxEntries - 1

	for server, users := range glusterClientCache {
		for user, searchClient := range users {
			gc := searchClient
			vols, err := gc.ListVolumes()
			if err != nil {
				glog.V(1).Infof("Failed to list volumes for %s / %s: %v", server, user, err)
				continue
			}

			for _, vol := range vols {
				entries = append(entries, &csi.ListVolumesResponse_Entry{Volume: vol})
			}
		}
	}

	resp := &csi.ListVolumesResponse{
		Entries:   entries[start:end],
		NextToken: fmt.Sprintf("%d", end),
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

//CreateSnapshot
func (cs *ControllerServer) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

//DeleteSnapshot
func (cs *ControllerServer) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

//ListSnapshots
func (cs *ControllerServer) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
