package glusterfs

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gluster/gluster-csi-driver/pkg/glusterfs/utils"

	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	"github.com/gluster/glusterd2/pkg/api"
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

// ControllerServer struct of Glusterfs CSI driver with supported methods of CSI controller server spec.
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
	csiConf  *CsiDrvParam
}

func convertVolumeParam(volumeString string) (int, error) {

	count, err := strconv.Atoi(volumeString)
	if err != nil {
		return 0, fmt.Errorf("failed to parse volumestring %q: %v", volumeString, err)
	}

	if count < 0 {
		return 0, fmt.Errorf("negative values are not allowed")
	}
	return count, nil
}

// ParseCreateVolRequest parse incoming volume create request and fill ProvisionerConfig.
func (cs *ControllerServer) ParseCreateVolRequest(req *csi.CreateVolumeRequest) (*ProvisionerConfig, error) {

	var reqConf ProvisionerConfig
	var gdReq api.VolCreateReq
	parseVolumeType := ""
	reqConf.gdVolReq = &gdReq

	// Get Volume name
	if req != nil {
		reqConf.gdVolReq.Name = req.Name
	}

	for k, v := range req.GetParameters() {
		switch strings.ToLower(k) {
		case "volumetype":
			if len(v) != 0 {
				parseVolumeType = v
			}
		default:
			return nil, fmt.Errorf("invalid option %q given for glusterfs CSI driver %s", k, glusterfsCSIDriverName)
		}
	}

	if len(parseVolumeType) != 0 {
		parseVolumeTypeInfo := strings.Split(parseVolumeType, ":")
		switch parseVolumeTypeInfo[0] {

		case "replicate":
			if len(parseVolumeTypeInfo) >= 2 {
				newReplicaCount, err := convertVolumeParam(parseVolumeTypeInfo[1])
				if err != nil {
					return nil, fmt.Errorf("error parsing volumeType %q: %s", parseVolumeTypeInfo[1], err)
				}
				reqConf.gdVolReq.ReplicaCount = newReplicaCount
			} else {
				reqConf.gdVolReq.ReplicaCount = defaultReplicaCount

			}
		case "disperse":
			if len(parseVolumeTypeInfo) >= 3 {
				newDisperseData, err := convertVolumeParam(parseVolumeTypeInfo[1])
				if err != nil {
					return nil, fmt.Errorf("error parsing volumeType %q: %s", parseVolumeTypeInfo[1], err)
				}
				newDisperseRedundancy, err := convertVolumeParam(parseVolumeTypeInfo[2])
				if err != nil {
					return nil, fmt.Errorf("error parsing volumeType %q: %s", parseVolumeTypeInfo[2], err)
				}
				reqConf.gdVolReq.DisperseDataCount = newDisperseData
				reqConf.gdVolReq.DisperseRedundancyCount = newDisperseRedundancy
			} else {
				reqConf.gdVolReq.DisperseCount = 1
			}
		default:
			return nil, fmt.Errorf("invalid option %q for glusterfs CSI driver %s", parseVolumeTypeInfo[0], glusterfsCSIDriverName)
		}

	}
	return &reqConf, nil
}

// CreateVolume creates and starts the volume
func (cs *ControllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	var glusterServer string
	var bkpServers []string

	glog.V(2).Infof("Request received %+v", req)

	if err := cs.validateCreateVolumeReq(req); err != nil {
		return nil, err
	}
	glog.V(1).Infof("creating volume with name : %s", req.Name)
	// If capacity mentioned, pick that or use default size 1 GB
	volSizeBytes := defaultVolumeSize
	if req.GetCapacityRange() != nil {
		volSizeBytes = int64(req.GetCapacityRange().GetRequiredBytes())
	}

	volSizeMB := int(utils.RoundUpSize(volSizeBytes, 1024*1024))

	// parse the request.
	parseResp, parseErr := cs.ParseCreateVolRequest(req)
	if parseErr != nil {
		return nil, status.Error(codes.InvalidArgument, "failed to parse request")
	}

	volumeName := parseResp.gdVolReq.Name

	glusterServer, bkpServers, err := cs.checkExistingVolume(volumeName, volSizeMB)
	if err != nil && err != errVolumeNotFound {
		return nil, err

	}
	if err == nil {
		resp := &csi.CreateVolumeResponse{
			Volume: &csi.Volume{
				Id:            volumeName,
				CapacityBytes: int64(volSizeBytes),
				Attributes: map[string]string{
					"glustervol":        volumeName,
					"glusterserver":     glusterServer,
					"glusterbkpservers": strings.Join(bkpServers, ":"),
				},
			},
		}
		return resp, nil
	}

	// If volume does not exist, provision volume
	glog.V(4).Infof("Received request to create/provision volume name:%s with size:%d", volumeName, volSizeMB)
	volMetaMap := make(map[string]string)
	volMetaMap[volumeOwnerAnn] = glusterfsCSIDriverName
	volumeReq := api.VolCreateReq{
		Name:         volumeName,
		Metadata:     volMetaMap,
		ReplicaCount: defaultReplicaCount,
		Size:         uint64(volSizeMB),
	}

	glog.V(2).Infof("volume request: %+v", volumeReq)

	volumeCreateResp, err := cs.client.VolumeCreate(volumeReq)
	if err != nil {
		glog.Errorf("failed to create volume : %v", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to create volume: %s", err.Error()))
	}

	glog.V(3).Infof("volume create response : %+v", volumeCreateResp)
	err = cs.client.VolumeStart(volumeName, true)
	if err != nil {
		//we dont need to delete the volume if volume start fails
		//as we are listing the volumes and starting it again
		//before sending back the response
		glog.Errorf("failed to start volume:%v", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to start volume %s", err.Error()))
	}

	glusterServer, bkpServers, err = cs.getClusterNodes()

	if err != nil {
		glog.Errorf("failed to fetch details of cluster nodes: %v", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("error in fecthing peer details %s", err.Error()))
	}

	resp := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			Id:            volumeName,
			CapacityBytes: int64(volSizeBytes),
			Attributes: map[string]string{
				"glustervol":        volumeName,
				"glusterserver":     glusterServer,
				"glusterbkpservers": strings.Join(bkpServers, ":"),
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

func (cs *ControllerServer) checkExistingVolume(volumeName string, volSizeMB int) (string, []string, error) {
	var (
		tspServers  []string
		mountServer string
		err         error
	)

	vol, err := cs.client.VolumeStatus(volumeName)
	if err != nil {
		glog.Errorf("failed to fetch volume : %v", err)
		errResp := cs.client.LastErrorResponse()
		//errResp will be nil in case of `No route to host` error
		if errResp != nil && errResp.StatusCode == http.StatusNotFound {
			return "", nil, errVolumeNotFound
		}
		return "", nil, status.Error(codes.Internal, fmt.Sprintf("error in fetching volume details %s", err.Error()))

	}

	// Do the owner validation
	if glusterAnnVal, found := vol.Info.Metadata[volumeOwnerAnn]; found {
		if glusterAnnVal != glusterfsCSIDriverName {
			return "", nil, status.Errorf(codes.Internal, "volume %s (%s) is not owned by Gluster CSI driver",
				vol.Info.Name, vol.Info.Metadata)
		}
	} else {
		return "", nil, status.Errorf(codes.Internal, "volume %s (%s) is not owned by Gluster CSI driver",
			vol.Info.Name, vol.Info.Metadata)
	}

	if int(vol.Size.Capacity) != volSizeMB {
		return "", nil, status.Error(codes.AlreadyExists, fmt.Sprintf("volume already exits with different size: %d", vol.Size.Capacity))
	}

	//volume has not started, start the volume
	if !vol.Online {
		err := cs.client.VolumeStart(vol.Info.Name, true)
		if err != nil {
			return "", nil, status.Error(codes.Internal, fmt.Sprintf("failed to start volume %s, err: %v", vol.Info.Name, err))
		}
	}

	glog.Info("Requested volume %s already exists in the gluster cluster", volumeName)
	mountServer, tspServers, err = cs.getClusterNodes()

	if err != nil {
		return "", nil, status.Error(codes.Internal, fmt.Sprintf("error in fetching backup/peer server details %s", err.Error()))
	}

	return mountServer, tspServers, nil
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
	glog.V(2).Infof("Gluster server and Backup servers [%+v,%+v]", glusterServer, bkpservers)

	return glusterServer, bkpservers, err
}

// DeleteVolume deletes the given volume.
func (cs *ControllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Volume delete request is nil")
	}

	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "volume ID is nil")
	}
	glog.V(2).Infof("Deleting volume with ID: %v", req.VolumeId)

	// Stop volume
	err := cs.client.VolumeStop(req.VolumeId)

	if err != nil {
		errResp := cs.client.LastErrorResponse()
		//errResp will be nil in case of `No route to host` error
		if errResp != nil && errResp.StatusCode == http.StatusNotFound {
			return &csi.DeleteVolumeResponse{}, nil
		}
		return nil, status.Errorf(codes.Internal, "failed to stop volume %s", err.Error())
	}

	// Delete volume
	err = cs.client.VolumeDelete(req.VolumeId)
	if err != nil {
		errResp := cs.client.LastErrorResponse()
		//errResp will be nil in case of No route to host error
		if errResp != nil && errResp.StatusCode == http.StatusNotFound {
			return &csi.DeleteVolumeResponse{}, nil
		}
		glog.Errorf("Deleting Volume %s failed: %v", req.VolumeId, err)
		return nil, status.Errorf(codes.Internal, "error deleting volume: %s", err.Error())
	}
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
		return nil, status.Errorf(codes.InvalidArgument, "volume capabilities request is nil")
	}

	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "ValidateVolumeCapabilities() - Volume ID is nil")
	}

	if req.VolumeCapabilities == nil {
		return nil, status.Error(codes.InvalidArgument, "ValidateVolumeCapabilities is nil")
	}
	_, err := cs.client.VolumeStatus(req.VolumeId)
	if err != nil {
		return nil, status.Error(codes.NotFound, "ValidateVolumeCapabilities() - Invalid Volume ID")
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
	glog.V(1).Infof("glusterfs CSI driver supported capabilities: %v", resp)
	return resp, nil
}

// ListVolumes returns a list of all requested volumes
func (cs *ControllerServer) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	//Fetch all the volumes in the TSP
	volumes, err := cs.client.Volumes("")
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	var entries []*csi.ListVolumesResponse_Entry
	for _, vol := range volumes {
		v, e := cs.client.VolumeStatus(vol.Name)
		if e != nil {
			return nil, status.Errorf(codes.Internal, "failed to get volume status %s", e.Error())
		}
		entries = append(entries, &csi.ListVolumesResponse_Entry{
			Volume: &csi.Volume{
				Id:            vol.Name,
				CapacityBytes: (int64(v.Size.Capacity)) * utils.MB,
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
