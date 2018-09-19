package glusterfs

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gluster/gluster-csi-driver/pkg/glusterfs/utils"

	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	"github.com/gluster/glusterd2/pkg/api"
	"github.com/gluster/glusterd2/pkg/restclient"
	"github.com/golang/glog"
	hcli "github.com/heketi/heketi/client/api/go-client"
	hapi "github.com/heketi/heketi/pkg/glusterfs/api"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type GlusterServerKind string

const (
	GD2DefaultInsecure = false

	HeketiDefaultBlock              = false
	HeketiDefaultReplica            = 3
	HeketiDefaultDisperseData       = 3
	HeketiDefaultDisperseRedundancy = 2
	HeketiDefaultGid                = 0
	HeketiDefaultSnapshot           = true
	HeketiDefaultSnapFactor         = 1.0

	ServerKindUnknown GlusterServerKind = ""
	ServerKindGD2     GlusterServerKind = "glusterd2"
	ServerKindHeketi  GlusterServerKind = "heketi"
)

type GD2Client struct {
	gd2Client    *restclient.Client
	cacert       string
	insecure     string
	insecureBool bool
}

type HeketiClient struct {
	heketiClient *hcli.Client
	clusterId    string
}

type GlusterClient struct {
	kind     GlusterServerKind
	url      string
	username string
	password string
	*GD2Client
	*HeketiClient
}

// GlusterClients is a map of maps of GlusterClient structs. The first map is
// indexed by the server/url the GlusterClient connects to. The second map is
// indexed by the username used on that server.
type GlusterClients map[string]map[string]*GlusterClient

var (
	glusterClientCache    = make(GlusterClients)
	glusterClientCacheMtx sync.Mutex
)

// SetPointerIfEmpty returns a new parameter if the old parameter is empty
func SetPointerIfEmpty(old, new interface{}) interface{} {
	if old == nil {
		return new
	}
	return old
}

// SetStringIfEmpty returns a new parameter if the old parameter is empty
func SetStringIfEmpty(old, new string) string {
	if len(old) == 0 {
		return new
	}
	return old
}

func ParseIntWithDefault(new string, defInt int) int {
	newInt := defInt

	if len(new) != 0 {
		parsedInt, err := strconv.Atoi(new)
		if err != nil {
			glog.Errorf("Bad int string [%s], using default [%s]", new, defInt)
		} else {
			newInt = parsedInt
		}
	}

	return newInt
}

func ParseBoolWithDefault(new string, defBool bool) bool {
	newBool := defBool

	if len(new) != 0 {
		parsedBool, err := strconv.ParseBool(new)
		if err != nil {
			glog.Errorf("Bad bool string [%s], using default [%s]", new, defBool)
		} else {
			newBool = parsedBool
		}
	}

	return newBool
}

func ParseFloatWithDefault(new string, defFloat float32) float32 {
	newFloat := defFloat

	if len(new) != 0 {
		parsedFloat, err := strconv.ParseFloat(new, 32)
		if err != nil {
			glog.Errorf("Bad float string [%s], using default [%s]", new, defFloat)
		} else {
			newFloat = float32(parsedFloat)
		}
	}

	return newFloat
}

func (gc *GlusterClient) gd2CheckRespErr(orig error) error {
	errResp := gc.gd2Client.LastErrorResponse()
	//errResp will be nil in case of No route to host error
	if errResp != nil && errResp.StatusCode == http.StatusNotFound {
		return errVolumeNotFound
	}
	return orig
}

func (gc *GlusterClient) gd2SetInsecure(new string) {
	gc.insecure = SetStringIfEmpty(gc.insecure, new)
	insecureBool, err := strconv.ParseBool(gc.insecure)
	if err != nil {
		glog.Errorf("Bad value [%s] for glusterd2insecure, using default [%s]", gc.insecure, GD2DefaultInsecure)
		gc.insecure = strconv.FormatBool(GD2DefaultInsecure)
		insecureBool = GD2DefaultInsecure
	}
	gc.insecureBool = insecureBool
}

func (gc *GlusterClient) heketiCheckRespErr(orig error) error {
	if orig.Error() == "Invalid path or request" {
		return errVolumeNotFound
	}
	return orig
}

func (gc *GlusterClient) detectServerKind() error {
	serverTestUrls := map[GlusterServerKind]string{
		ServerKindGD2:    gc.url + "/ping",
		ServerKindHeketi: gc.url + "/hello",
	}

	for serverKind, testUrl := range serverTestUrls {
		if resp, err := http.Get(testUrl); err == nil {
			if resp.StatusCode == 200 {
				gc.kind = serverKind
			}
			resp.Body.Close()
		} else if err != nil {
			glog.V(1).Infof("Error detecting %s server at %s: %v", serverKind, testUrl, err)
		}
	}

	if gc.kind == ServerKindUnknown {
		return fmt.Errorf("Failed to detect server %s kind", gc.url)
	}
	glog.V(1).Infof("Server %s kind detected: %s", gc.url, gc.kind)
	return nil
}

func (gc *GlusterClient) GetClusterNodes(volumeId string) ([]string, error) {
	glusterServers := []string{}

	switch kind := gc.kind; kind {
	case ServerKindGD2:
		client := gc.gd2Client
		peers, err := client.Peers()
		if err != nil {
			return nil, err
		}

		for _, p := range peers {
			for _, a := range p.PeerAddresses {
				ip := strings.Split(a, ":")
				glusterServers = append(glusterServers, ip[0])
			}
		}
	case ServerKindHeketi:
		client := gc.heketiClient
		clr, err := client.ClusterList()
		if err != nil {
			return nil, fmt.Errorf("failed to list heketi clusters: %v", err)
		}
		cluster := clr.Clusters[0]
		clusterInfo, err := client.ClusterInfo(cluster)
		if err != nil {
			return nil, fmt.Errorf("Failed to get cluster %s details: %v", cluster, err)
		}

		for _, node := range clusterInfo.Nodes {
			nodeInfo, err := client.NodeInfo(string(node))
			if err != nil {
				return nil, fmt.Errorf("failed to get node %s info: %v", string(node), err)
			}
			nodeAddr := strings.Join(nodeInfo.NodeAddRequest.Hostnames.Storage, "")
			glusterServers = append(glusterServers, nodeAddr)
		}
	default:
		return nil, fmt.Errorf("Invalid server kind: %s", gc.kind)
	}

	if len(glusterServers) == 0 {
		return nil, fmt.Errorf("No hosts found for %s / %s", gc.url, gc.username)
	}
	glog.V(2).Infof("Gluster servers: %+v", glusterServers)

	return glusterServers, nil
}

func (gc *GlusterClient) CheckExistingVolume(volumeId string, volSizeBytes int64) error {
	switch kind := gc.kind; kind {
	case ServerKindGD2:
		client := gc.gd2Client
		vol, err := client.VolumeStatus(volumeId)
		if err != nil {
			return gc.gd2CheckRespErr(err)
		}

		// Do the owner validation
		if glusterAnnVal, found := vol.Info.Metadata[volumeOwnerAnn]; !found || glusterAnnVal != glusterfsCSIDriverName {
			return fmt.Errorf("volume %s is not owned by %s", vol.Info.Name, glusterfsCSIDriverName)
		}

		// Check requested capacity is the same as existing capacity
		if volSizeBytes > 0 && vol.Size.Capacity != uint64(utils.RoundUpToMiB(volSizeBytes)) {
			return fmt.Errorf("volume %s already exists with different size: %d MiB", vol.Info.Name, vol.Size.Capacity)
		}

		// If volume not started, start the volume
		if !vol.Online {
			err := client.VolumeStart(vol.Info.Name, true)
			if err != nil {
				return fmt.Errorf("failed to start volume %s", vol.Info.Name)
			}
		}
	case ServerKindHeketi:
		var vol *csi.Volume
		vols, err := gc.ListVolumes()
		if err != nil {
			return fmt.Errorf("Error listing volumes: %v", err)
		}

		for _, volEnt := range vols {
			if volEnt.Id == volumeId {
				vol = volEnt
			}
		}
		if vol == nil {
			return errVolumeNotFound
		}

		if volSizeBytes > 0 && utils.RoundUpToGiB(vol.CapacityBytes) != utils.RoundUpToGiB(volSizeBytes) {
			return fmt.Errorf("volume %s already exists with different size: %d GiB", volumeId, utils.RoundUpToGiB(vol.CapacityBytes))
		}
	default:
		return fmt.Errorf("Invalid server kind: %s", kind)
	}

	glog.V(1).Infof("Found volume %s in the storage pool", volumeId)
	return nil
}

func (gc *GlusterClient) CreateVolume(volumeName string, volSizeBytes int64, params map[string]string) error {
	switch kind := gc.kind; kind {
	case ServerKindGD2:
		client := gc.gd2Client
		volMetaMap := make(map[string]string)
		volMetaMap[volumeOwnerAnn] = glusterfsCSIDriverName
		volumeReq := api.VolCreateReq{
			Name:         volumeName,
			Metadata:     volMetaMap,
			ReplicaCount: defaultReplicaCount,
			Size:         uint64(utils.RoundUpToMiB(volSizeBytes)),
		}

		glog.V(2).Infof("volume create request: %+v", volumeReq)
		volumeCreateResp, err := client.VolumeCreate(volumeReq)
		if err != nil {
			return fmt.Errorf("failed to create volume %s: %v", volumeName, err)
		}

		glog.V(3).Infof("volume create response: %+v", volumeCreateResp)
		err = client.VolumeStart(volumeName, true)
		if err != nil {
			//we dont need to delete the volume if volume start fails
			//as we are listing the volumes and starting it again
			//before sending back the response
			return fmt.Errorf("failed to start volume %s: %v", volumeName, err)
		}
	case ServerKindHeketi:
		client := gc.heketiClient

		durabilityInfo := hapi.VolumeDurabilityInfo{Type: hapi.DurabilityReplicate, Replicate: hapi.ReplicaDurability{Replica: HeketiDefaultReplica}}
		volumeType := params["glustervolumetype"]
		if len(volumeType) != 0 {
			volumeTypeList := strings.Split(volumeType, ":")

			switch volumeTypeList[0] {
			case "replicate":
				if len(volumeTypeList) >= 2 {
					durabilityInfo.Replicate.Replica = ParseIntWithDefault(volumeTypeList[1], HeketiDefaultReplica)
				}
			case "disperse":
				if len(volumeTypeList) >= 3 {
					data := ParseIntWithDefault(volumeTypeList[1], HeketiDefaultDisperseData)
					redundancy := ParseIntWithDefault(volumeTypeList[2], HeketiDefaultDisperseRedundancy)
					durabilityInfo = hapi.VolumeDurabilityInfo{Type: hapi.DurabilityEC, Disperse: hapi.DisperseDurability{Data: data, Redundancy: redundancy}}
				} else {
					return fmt.Errorf("Volume type 'disperse' must have format: 'disperse:<data>:<redundancy>'")
				}
			case "none":
				durabilityInfo = hapi.VolumeDurabilityInfo{Type: hapi.DurabilityDistributeOnly}
			default:
				return fmt.Errorf("Invalid volume type %s", volumeType)
			}
		}

		volumeReq := &hapi.VolumeCreateRequest{
			Size:                 int(utils.RoundUpToGiB(volSizeBytes)),
			Name:                 volumeName,
			Durability:           durabilityInfo,
			Gid:                  int64(ParseIntWithDefault(params["glustergid"], HeketiDefaultGid)),
			GlusterVolumeOptions: strings.Split(params["glustervolumeoptions"], ","),
			Block:                ParseBoolWithDefault(params["glusterblockhost"], HeketiDefaultBlock),
			Snapshot: struct {
				Enable bool    `json:"enable"`
				Factor float32 `json:"factor"`
			}{
				Enable: ParseBoolWithDefault(params["glustersnapshot"], HeketiDefaultSnapshot),
				Factor: ParseFloatWithDefault(params["glustersnapfactor"], HeketiDefaultSnapFactor),
			},
		}

		_, err := client.VolumeCreate(volumeReq)
		if err != nil {
			return fmt.Errorf("Failed to create volume %s: %v", volumeName, err)
		}
	default:
		return fmt.Errorf("Invalid server kind: %s", kind)
	}

	return nil
}

func (gc *GlusterClient) DeleteVolume(volumeId string) error {
	switch kind := gc.kind; kind {
	case ServerKindGD2:
		client := gc.gd2Client
		err := client.VolumeStop(volumeId)
		if err != nil {
			return gc.gd2CheckRespErr(err)
		}

		err = client.VolumeDelete(volumeId)
		if err != nil {
			return gc.gd2CheckRespErr(err)
		}
	case ServerKindHeketi:
		var vol *csi.Volume
		vols, err := gc.ListVolumes()
		if err != nil {
			return fmt.Errorf("Error listing volumes: %v", err)
		}

		for _, volEnt := range vols {
			if volEnt.Id == volumeId {
				vol = volEnt
			}
		}
		if vol == nil {
			return errVolumeNotFound
		}

		client := gc.heketiClient
		err = client.VolumeDelete(vol.Attributes["glustervolheketiid"])
		if err != nil {
			return gc.heketiCheckRespErr(err)
		}
	default:
		return fmt.Errorf("Invalid server kind: %s", gc.kind)
	}

	return nil
}

func (gc GlusterClient) ListVolumes() ([]*csi.Volume, error) {
	volumes := []*csi.Volume{}

	switch kind := gc.kind; kind {
	case ServerKindGD2:
		client := gc.gd2Client

		vols, err := client.Volumes("")
		if err != nil {
			return nil, err
		}

		for _, vol := range vols {
			v, err := client.VolumeStatus(vol.Name)
			if err != nil {
				glog.V(1).Infof("Error getting volume %s status: %s", vol.Name, err)
				continue
			}
			volumes = append(volumes, &csi.Volume{
				Id:            vol.Name,
				CapacityBytes: (int64(v.Size.Capacity)) * utils.MB,
			})
		}
	case ServerKindHeketi:
		client := gc.heketiClient
		vols, err := client.VolumeList()
		if err != nil {
			return nil, err
		}
		for _, volId := range vols.Volumes {
			vol, err := client.VolumeInfo(volId)
			if err != nil {
				return nil, err
			}
			volumes = append(volumes, &csi.Volume{
				Id:            vol.VolumeInfo.VolumeCreateRequest.Name,
				CapacityBytes: (int64(vol.VolumeCreateRequest.Size)) * utils.GiB,
				Attributes: map[string]string{
					"glustervolheketiid": vol.VolumeInfo.Id,
				},
			})
		}
	default:
		return nil, fmt.Errorf("Invalid server kind: %s", gc.kind)
	}

	return volumes, nil
}

func (gcc GlusterClients) Get(server, user string) (*GlusterClient, error) {
	glusterClientCacheMtx.Lock()
	defer glusterClientCacheMtx.Unlock()

	users, ok := gcc[server]
	if !ok {
		return nil, fmt.Errorf("Server %s not found in cache", server)
	}
	gc, ok := users[user]
	if !ok {
		return nil, fmt.Errorf("Client %s / %s not found in cache", server, user)
	}

	return gc, nil
}

// FindVolumeClient looks for a volume among current known servers
func (gcc GlusterClients) FindVolumeClient(volumeId string) (*GlusterClient, error) {
	var gc *GlusterClient

	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	// Search all PVs for volumes from us
	pvs, err := clientset.CoreV1().PersistentVolumes().List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	pvList := []corev1.PersistentVolume{}
	for i, pv := range pvs.Items {
		if pv.Spec.CSI != nil && pv.Spec.CSI.Driver == glusterfsCSIDriverName {
			pvList = append(pvList, pv)

			if pv.Spec.CSI.VolumeHandle == volumeId {
				vol := pv.Spec.CSI
				url := vol.VolumeAttributes["glusterurl"]
				user := vol.VolumeAttributes["glusteruser"]
				gc, err = gcc.Get(url, user)
				if err != nil {
					glog.V(1).Infof(" Error getting GlusterClient %s / %s: %s", url, user, err)
					continue
				}
				break
			}
		}
		if i == len(pvs.Items)-1 {
			glog.Warningf("No PV found for volume %s", volumeId)
		}
	}

	// If volume not found, cycle through all discovered connections
	if gc == nil {
		// Update GlusterClient cache
		for _, pv := range pvList {
			attrs := pv.Spec.CSI.VolumeAttributes
			url := attrs["glusterurl"]
			user := attrs["glusteruser"]
			_, err := gcc.Get(url, user)
			if err != nil {
				glog.V(1).Infof("GlusterClient for %s / %s not found, initializing", url, user)

				searchClient := &GlusterClient{
					kind:     GlusterServerKind(attrs["glusterserverkind"]),
					url:      url,
					username: user,
					password: attrs["glusterusersecret"],
				}
				err = gcc.Init(searchClient)
				if err != nil {
					glog.V(1).Infof("Error initializing GlusterClient %s / %s: %s", url, user, err)
					continue
				}
			}
		}

		// Check every connection for the volume
		for server, users := range gcc {
			for user, searchClient := range users {
				err = searchClient.CheckExistingVolume(volumeId, 0)
				if err != nil {
					glog.V(1).Infof("Error with GlusterClient %s / %s: %s", server, user, err)
					continue
				}

				gc = searchClient
				break
			}
			if gc != nil {
				break
			}
		}
	}

	if gc == nil {
		return nil, errVolumeNotFound
	}
	return gc, nil
}

func (gcc GlusterClients) Init(client *GlusterClient) error {
	var err error

	glusterClientCacheMtx.Lock()
	defer glusterClientCacheMtx.Unlock()

	srvEnt, ok := gcc[client.url]
	if !ok {
		gcc[client.url] = make(map[string]*GlusterClient)
		srvEnt = gcc[client.url]
	}
	usrEnt, ok := srvEnt[client.username]
	if ok {
		client.kind = GlusterServerKind(SetStringIfEmpty(string(client.kind), string(usrEnt.kind)))
		client.password = SetStringIfEmpty(client.password, usrEnt.password)
		switch kind := client.kind; kind {
		case ServerKindGD2:
			if client.GD2Client == nil {
				client.GD2Client = &GD2Client{insecureBool: GD2DefaultInsecure}
			}
			client.GD2Client = SetPointerIfEmpty(client.GD2Client, usrEnt.GD2Client).(*GD2Client)
			client.cacert = SetStringIfEmpty(client.cacert, usrEnt.cacert)
			client.gd2SetInsecure(usrEnt.insecure)
		case ServerKindHeketi:
			if client.HeketiClient == nil {
				client.HeketiClient = &HeketiClient{}
			}
			client.HeketiClient = SetPointerIfEmpty(client.HeketiClient, usrEnt.HeketiClient).(*HeketiClient)
		default:
			err = fmt.Errorf("invalid server kind: %s", client.kind)
		}
	} else {
		glog.V(1).Infof("REST client %s/%s not found in cache, initializing", client.url, client.username)

		if client.kind == ServerKindUnknown {
			err = client.detectServerKind()
			if err != nil {
				return err
			}
		}

		switch kind := client.kind; kind {
		case ServerKindGD2:
			if client.GD2Client == nil {
				client.GD2Client = &GD2Client{insecureBool: GD2DefaultInsecure}
			}
			gd2, err := restclient.New(client.url, client.username, client.password, client.cacert, client.insecureBool)
			if err != nil {
				return fmt.Errorf("failed to create %s REST client: %s", client.kind, err)
			}
			client.gd2Client = gd2
		case ServerKindHeketi:
			if client.HeketiClient == nil {
				client.HeketiClient = &HeketiClient{}
			}
			client.heketiClient = hcli.NewClient(client.url, client.username, client.password)
		default:
			return fmt.Errorf("invalid server kind: %s", client.kind)
		}
	}

	srvEnt[client.username] = client

	return nil
}
