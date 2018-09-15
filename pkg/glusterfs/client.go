package glusterfs

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gluster/glusterd2/pkg/restclient"
	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	GD2DefaultInsecure = false
)

type GlusterClient struct {
	url          string
	username     string
	password     string
	client       *restclient.Client
	cacert       string
	insecure     string
	insecureBool bool
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

func (gc *GlusterClient) setInsecure(new string) {
	gc.insecure = SetStringIfEmpty(gc.insecure, new)
	insecureBool, err := strconv.ParseBool(gc.insecure)
	if err != nil {
		glog.Errorf("Bad value [%s] for glusterd2insecure, using default [%s]", gc.insecure, GD2DefaultInsecure)
		gc.insecure = strconv.FormatBool(GD2DefaultInsecure)
		insecureBool = GD2DefaultInsecure
	}
	gc.insecureBool = insecureBool
}

func (gc *GlusterClient) GetClusterNodes() (string, []string, error) {
	glusterServer := ""
	bkpservers := []string{}

	peers, err := gc.client.Peers()
	if err != nil {
		return "", nil, err
	}

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

	return glusterServer, bkpservers, nil
}

func (gc *GlusterClient) CheckExistingVolume(volumeId string, volSizeMB uint64) error {
	vol, err := gc.client.VolumeStatus(volumeId)
	if err != nil {
		errResp := gc.client.LastErrorResponse()
		//errResp will be nil in case of No route to host error
		if errResp != nil && errResp.StatusCode == http.StatusNotFound {
			return errVolumeNotFound
		}
		return err
	}

	// Do the owner validation
	if glusterAnnVal, found := vol.Info.Metadata[glusterDescAnn]; !found || glusterAnnVal != glusterDescAnnValue {
		return fmt.Errorf("volume %s is not owned by %s", vol.Info.Name, glusterDescAnnValue)
	}

	// Check requested capacity is the same as existing capacity
	if volSizeMB > 0 && vol.Size.Capacity != volSizeMB {
		return fmt.Errorf("volume %s already exists with different size: %d", vol.Info.Name, vol.Size.Capacity)
	}

	// If volume not started, start the volume
	if !vol.Online {
		err := gc.client.VolumeStart(vol.Info.Name, true)
		if err != nil {
			return fmt.Errorf("failed to start volume %s", vol.Info.Name)
		}
	}

	glog.Info("Found volume %s in the storage pool", volumeId)

	return nil
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
		if pv.Spec.CSI != nil && pv.Spec.CSI.Driver == driverName {
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

	if gc.client == nil {
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
		client.password = SetStringIfEmpty(client.password, usrEnt.password)
		client.client = SetPointerIfEmpty(client.client, usrEnt.client).(*restclient.Client)
		client.cacert = SetStringIfEmpty(client.cacert, usrEnt.cacert)
		client.setInsecure(usrEnt.insecure)
	} else {
		glog.V(1).Infof("REST client %s / %s not found in cache, initializing", client.url, client.username)

		gd2, err := restclient.New(client.url, client.username, client.password, client.cacert, client.insecureBool)
		if err == nil {
			client.client = gd2
		} else {
			return fmt.Errorf("Failed to create REST client %s / %s: %s", err)
		}
	}

	srvEnt[client.username] = client

	return nil
}
