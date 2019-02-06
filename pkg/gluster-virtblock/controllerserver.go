package glustervirtblock

import (
	"context"
	"net/http"
	"strings"

	"github.com/gluster/gluster-csi-driver/pkg/utils"

	"github.com/container-storage-interface/spec/lib/go/csi"
	bapi "github.com/gluster/glusterd2/plugins/blockvolume/api"
	"github.com/kubernetes-csi/csi-lib-utils/protosanitizer"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog"
)

const (
	defaultVolumeSize int64  = 1000 * utils.MB // default volume size ie 1 GB
	virtBlockProvider string = "virtblock"
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

// CreateVolume creates and starts the volume
func (cs *ControllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	klog.V(2).Infof("request received %+v", protosanitizer.StripSecrets(req))

	if err := cs.validateCreateVolumeReq(req); err != nil {
		return nil, err
	}

	klog.V(1).Infof("creating volume with name %s", req.Name)

	volumeName := req.Name
	volSizeBytes := cs.getVolumeSize(req)

	if req.VolumeContentSource.GetSnapshot().GetSnapshotId() != "" {
		return nil, status.Errorf(codes.Internal, "creating volume from snapshot is not supported. volume: %s", volumeName)
	}

	blockVolInfo := bapi.BlockVolumeInfo{
		Name: volumeName,
		Size: uint64(volSizeBytes),
	}
	volumeReq := bapi.BlockVolumeCreateRequest{
		BlockVolumeInfo: &blockVolInfo,
	}

	klog.V(2).Infof("block volume create request: %+v", volumeReq)
	blockVolCreateResp, err := cs.client.BlockVolumeCreate(virtBlockProvider, volumeReq)
	if err != nil {
		klog.Errorf("failed to create block volume: %s err: %v", volumeName, err)
		errResp := cs.client.LastErrorResponse()
		// errResp will be nil in case of `No route to host` error
		if errResp != nil && errResp.StatusCode == http.StatusConflict {
			return nil, status.Errorf(codes.Aborted, "block volume create already in process: %v", err)
		}

		return nil, status.Errorf(codes.Internal, "failed to create block volume: %s err: %v", volumeName, err)
	}

	glusterServer, bkpServers, err := utils.GetClusterNodes(cs.client)
	if err != nil {
		klog.Errorf("failed to get cluster nodes: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to get cluster nodes: %v", err)
	}

	klog.V(3).Infof("blockvolume create response : %+v", blockVolCreateResp)
	resp := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      volumeName,
			CapacityBytes: volSizeBytes,
			VolumeContext: map[string]string{
				"glustervol":        blockVolCreateResp.HostingVolume,
				"glusterserver":     glusterServer,
				"glusterbkpservers": strings.Join(bkpServers, ":"),
			},
		},
	}

	klog.V(4).Infof("CSI block volume response: %+v", protosanitizer.StripSecrets(resp))
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

// DeleteVolume deletes the given volume.
func (cs *ControllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "block volume delete request is nil")
	}

	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "volume ID is nil")
	}
	klog.V(2).Infof("deleting block volume with ID: %s", volumeID)

	// Delete volume
	err := cs.client.BlockVolumeDelete(virtBlockProvider, volumeID)
	if err != nil {
		errResp := cs.client.LastErrorResponse()
		// errResp will be nil in case of No route to host error
		if errResp != nil && errResp.StatusCode == http.StatusNotFound {
			return &csi.DeleteVolumeResponse{}, nil
		}
		klog.Errorf("deleting block volume %s failed: %v", req.VolumeId, err)
		return nil, status.Errorf(codes.Internal, "deleting volume %s failed: %v", req.VolumeId, err)
	}

	klog.Infof("successfully deleted block volume %s", volumeID)
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

	_, err := cs.client.BlockVolumeGet(virtBlockProvider, volumeID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "ValidateVolumeCapabilities() - %v", err)
	}

	var vcaps []*csi.VolumeCapability_AccessMode
	for _, mode := range []csi.VolumeCapability_AccessMode_Mode{
		csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
		csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY,
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

	klog.V(1).Infof("GlusterFS Block CSI driver volume capabilities: %+v", resp)
	return resp, nil
}

// ListVolumes returns a list of volumes
func (cs *ControllerServer) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	var entries []*csi.ListVolumesResponse_Entry

	volumes, err := cs.client.BlockVolumeList(virtBlockProvider)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	for _, vol := range volumes {
		entries = append(entries, &csi.ListVolumesResponse_Entry{
			Volume: &csi.Volume{
				VolumeId:      vol.Name,
				CapacityBytes: int64(vol.Size),
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
	} {
		caps = append(caps, newCap(cap))
	}

	resp := &csi.ControllerGetCapabilitiesResponse{
		Capabilities: caps,
	}

	return resp, nil
}
