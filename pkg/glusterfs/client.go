package glusterfs

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	cli "github.com/gluster/gluster-csi-driver/pkg/client"
	"github.com/gluster/gluster-csi-driver/pkg/command"

	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	"github.com/gluster/glusterd2/pkg/api"
	gd2Error "github.com/gluster/glusterd2/pkg/errors"
	"github.com/gluster/glusterd2/pkg/restclient"
	"github.com/golang/glog"
)

const (
	gd2DefaultInsecure  = "false"
	volumeOwnerAnn      = "VolumeOwner"
	defaultReplicaCount = 3
)

type gd2Client struct {
	client       *restclient.Client
	driverName   string
	url          string
	username     string
	password     string
	caCert       string
	insecure     string
	insecureBool bool
}

// NewClient returns a new gd2Client
func NewClient(config *command.Config) (cli.GlusterClient, error) {
	var err error

	client := gd2Client{
		driverName: config.Name,
		url:        config.RestURL,
		username:   config.RestUser,
		password:   config.RestSecret,
	}

	client.insecure, client.insecureBool = client.setInsecure(gd2DefaultInsecure)
	client, err = client.setClient(client)

	return client, err
}

func (gc gd2Client) checkRespErr(orig error, kind, name string) error {
	errResp := gc.client.LastErrorResponse()
	//errResp will be nil in case of No route to host error
	if errResp != nil && errResp.StatusCode == http.StatusNotFound {
		return cli.ErrNotFound(kind, name)
	}
	return orig
}

func (gc gd2Client) setInsecure(new string) (string, bool) {
	insecure := cli.SetStringIfEmpty(gc.insecure, new)
	insecureBool, err := strconv.ParseBool(gc.insecure)
	if err != nil {
		glog.Errorf("bad value [%s] for glusterd2insecure, using default [%s]", gc.insecure, gd2DefaultInsecure)
		insecure = gd2DefaultInsecure
		insecureBool, err = strconv.ParseBool(gd2DefaultInsecure)
		if err != nil {
			panic(err)
		}
	}

	return insecure, insecureBool
}

func (gc gd2Client) setClient(client gd2Client) (gd2Client, error) {
	if gc.client == nil {
		gd2, err := restclient.New(client.url, client.username, client.password, client.caCert, client.insecureBool)
		if err != nil {
			return gd2Client{}, fmt.Errorf("failed to create glusterd2 REST client: %s", err)
		}
		err = gd2.Ping()
		if err != nil {
			return gd2Client{}, fmt.Errorf("error finding glusterd2 server at %s: %v", gc.url, err)
		}

		gc.client = gd2
	}

	client.client = gc.client
	return client, nil
}

// GetClusterNodes retrieves a list of cluster peer nodes
func (gc gd2Client) GetClusterNodes(volumeID string) ([]string, error) {
	glusterServers := []string{}

	peers, err := gc.client.Peers()
	if err != nil {
		return nil, err
	}

	for _, p := range peers {
		for _, a := range p.PeerAddresses {
			ip := strings.Split(a, ":")
			glusterServers = append(glusterServers, ip[0])
		}
	}

	if len(glusterServers) == 0 {
		return nil, fmt.Errorf("no hosts found for %s / %s", gc.url, gc.username)
	}

	return glusterServers, nil
}

// CheckExistingVolume checks whether a given volume already exists
func (gc gd2Client) CheckExistingVolume(volumeID string, volSizeBytes int64) error {
	vol, err := gc.client.Volumes(volumeID)
	if err != nil {
		return gc.checkRespErr(err, "volume", volumeID)
	}

	volInfo := vol[0]
	// Do the owner validation
	if glusterAnnVal, found := volInfo.Metadata[volumeOwnerAnn]; !found || glusterAnnVal != gc.driverName {
		return cli.ErrExists("volume", volInfo.Name, "owner", glusterAnnVal)
	}

	// Check requested capacity is the same as existing capacity
	if volSizeBytes > 0 && volInfo.Capacity != uint64(cli.RoundUpToMiB(volSizeBytes)) {
		return cli.ErrExists("volume", volInfo.Name, "size (MiB)", strconv.FormatUint(volInfo.Capacity, 10))
	}

	// If volume not started, start the volume
	if volInfo.State != api.VolStarted {
		err := gc.client.VolumeStart(volInfo.Name, true)
		if err != nil {
			return fmt.Errorf("failed to start volume %s: %v", volInfo.Name, err)
		}
	}

	glog.Infof("requested volume %s already exists in the gluster cluster", volumeID)

	return nil
}

// CreateVolume creates a new volume
func (gc gd2Client) CreateVolume(volumeName string, volSizeBytes int64, params map[string]string) error {
	volMetaMap := make(map[string]string)
	volMetaMap[volumeOwnerAnn] = gc.driverName
	volumeReq := api.VolCreateReq{
		Name:         volumeName,
		Metadata:     volMetaMap,
		ReplicaCount: defaultReplicaCount,
		Size:         uint64(cli.RoundUpToMiB(volSizeBytes)),
	}

	glog.V(2).Infof("volume create request: %+v", volumeReq)
	volumeCreateResp, err := gc.client.VolumeCreate(volumeReq)
	if err != nil {
		return fmt.Errorf("failed to create volume %s: %v", volumeName, err)
	}

	glog.V(3).Infof("volume create response: %+v", volumeCreateResp)
	err = gc.client.VolumeStart(volumeName, true)
	if err != nil {
		//we dont need to delete the volume if volume start fails
		//as we are listing the volumes and starting it again
		//before sending back the response
		return fmt.Errorf("failed to start volume %s: %v", volumeName, err)
	}

	return nil
}

// DeleteVolume deletes a volume
func (gc gd2Client) DeleteVolume(volumeID string) error {
	err := gc.client.VolumeStop(volumeID)
	if err != nil && err.Error() != gd2Error.ErrVolAlreadyStopped.Error() {
		return gc.checkRespErr(err, "volume", volumeID)
	}

	err = gc.client.VolumeDelete(volumeID)
	if err != nil {
		return gc.checkRespErr(err, "volume", volumeID)
	}

	return nil
}

// ListVolumes lists all volumes in the cluster
func (gc gd2Client) ListVolumes() ([]*csi.Volume, error) {
	volumes := []*csi.Volume{}

	vols, err := gc.client.Volumes("")
	if err != nil {
		return nil, err
	}

	for _, vol := range vols {
		v, err := gc.client.VolumeStatus(vol.Name)
		if err != nil {
			glog.V(1).Infof("error getting volume %s status: %s", vol.Name, err)
			continue
		}
		volumes = append(volumes, &csi.Volume{
			Id:            vol.Name,
			CapacityBytes: (int64(v.Size.Capacity)) * cli.MB,
		})
	}

	return volumes, nil
}

func (gc *gd2Client) csiSnap(snap api.SnapInfo) *csi.Snapshot {
	return &csi.Snapshot{
		Id:             snap.VolInfo.Name,
		SourceVolumeId: snap.ParentVolName,
		CreatedAt:      snap.CreatedAt.Unix(),
		SizeBytes:      (int64(snap.VolInfo.Capacity)) * cli.MB,
		Status: &csi.SnapshotStatus{
			Type: csi.SnapshotStatus_READY,
		},
	}
}

// CheckExistingSnapshot checks if a snapshot exists in the TSP
func (gc gd2Client) CheckExistingSnapshot(snapName, volName string) (*csi.Snapshot, error) {
	snapInfo, err := gc.client.SnapshotInfo(snapName)
	if err != nil {
		return nil, gc.checkRespErr(err, "snapshot", snapName)
	}
	if len(volName) != 0 && snapInfo.ParentVolName != volName {
		return nil, cli.ErrExists("snapshot", snapName, "parent volume", snapInfo.ParentVolName)
	}

	if snapInfo.VolInfo.State != api.VolStarted {
		actReq := api.SnapActivateReq{
			Force: true,
		}
		err = gc.client.SnapshotActivate(actReq, snapName)
		if err != nil {
			return nil, fmt.Errorf("failed to activate snapshot: %v", err)
		}
	}

	return gc.csiSnap(api.SnapInfo(snapInfo)), nil
}

// CreateSnapshot creates a snapshot of an existing volume
func (gc gd2Client) CreateSnapshot(snapName, srcVol string) (*csi.Snapshot, error) {
	snapReq := api.SnapCreateReq{
		VolName:  srcVol,
		SnapName: snapName,
		Force:    true,
	}
	glog.V(2).Infof("snapshot request: %+v", snapReq)
	snapInfo, err := gc.client.SnapshotCreate(snapReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot %s: %v", snapName, err)
	}

	err = gc.client.SnapshotActivate(api.SnapActivateReq{Force: true}, snapName)
	if err != nil {
		return nil, fmt.Errorf("failed to activate snapshot %s: %v", snapName, err)
	}

	return gc.csiSnap(api.SnapInfo(snapInfo)), nil
}

// CloneSnapshot creates a clone of a snapshot
func (gc gd2Client) CloneSnapshot(snapName, volName string) error {
	var snapreq api.SnapCloneReq

	glog.V(2).Infof("creating volume from snapshot %s", snapName)
	snapreq.CloneName = volName
	snapResp, err := gc.client.SnapshotClone(snapName, snapreq)
	if err != nil {
		return fmt.Errorf("failed to create volume clone: %v", err)
	}
	err = gc.client.VolumeStart(volName, true)
	if err != nil {
		return fmt.Errorf("failed to start volume: %v", err)
	}
	glog.V(1).Infof("snapshot clone response: %+v", snapResp)
	return nil
}

// DeleteSnapshot deletes a snapshot
func (gc gd2Client) DeleteSnapshot(snapName string) error {
	err := gc.client.SnapshotDeactivate(snapName)
	if err != nil {
		//if errResp != nil && errResp.StatusCode != http.StatusNotFound && err.Error() != gd2Error.ErrSnapDeactivated.Error() {
		return gc.checkRespErr(err, "snapshot", snapName)
	}
	err = gc.client.SnapshotDelete(snapName)
	if err != nil {
		return gc.checkRespErr(err, "snapshot", snapName)
	}
	return nil
}

// ListSnapshots lists all snapshots
func (gc gd2Client) ListSnapshots(snapName, srcVol string) ([]*csi.Snapshot, error) {
	var snaps []*csi.Snapshot
	var snaplist api.SnapListResp

	if len(snapName) != 0 {
		// Get snapshot by name
		snap, err := gc.client.SnapshotInfo(snapName)
		if err == nil {
			snapInfo := api.SnapInfo(snap)
			snaplist = append(snaplist, api.SnapList{ParentName: snapInfo.ParentVolName, SnapList: []api.SnapInfo{snapInfo}})
		}
	} else if len(srcVol) != 0 {
		// Get all snapshots for source volume
		snaps, err := gc.client.SnapshotList(srcVol)
		if err != nil {
			errResp := gc.client.LastErrorResponse()
			if errResp != nil && errResp.StatusCode != http.StatusNotFound {
				return nil, fmt.Errorf("[%s/%s]: %v", gc.url, gc.username, err)
			}
		}
		snaplist = append(snaplist, snaps...)
	} else {
		// Get all snapshots in TSP
		snaps, err := gc.client.SnapshotList("")
		if err != nil {
			return nil, fmt.Errorf("[%s/%s]: %v", gc.url, gc.username, err)
		}
		snaplist = append(snaplist, snaps...)
	}

	for _, snap := range snaplist {
		for _, s := range snap.SnapList {
			snaps = append(snaps, gc.csiSnap(s))
		}

	}
	return snaps, nil
}
