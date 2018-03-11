/*
Copyright 2018 The Gluster CSI Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package glusterfs

import (
	"fmt"

	dstrings "strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/glog"
	gcli "github.com/heketi/heketi/client/api/go-client"
	gapi "github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/kubernetes-csi/drivers/pkg/csi-common"
	"github.com/pborman/uuid"
	"golang.org/x/net/context"
	"k8s.io/kubernetes/pkg/volume"
)

const (
	defaultGlusterURL    = "http://127.0.0.1:8081"
	defaultGlusterServer = "127.0.0.1"
)

type controllerServer struct {
	*csicommon.DefaultControllerServer
}

func GetVersionString(ver *csi.Version) string {
	return fmt.Sprintf("%d.%d.%d", ver.Major, ver.Minor, ver.Patch)
}

func (cs *controllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {

	if err := cs.Driver.ValidateControllerServiceRequest(req.Version, csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME); err != nil {
		glog.V(3).Infof("invalid create volume req: %v", req)
		return nil, err
	}

	// Volume Size - Default is 1 GiB
	volSizeBytes := int64(1 * 1024 * 1024 * 1024)
	if req.GetCapacityRange() != nil {
		volSizeBytes = int64(req.GetCapacityRange().GetRequiredBytes())
	}
	volSizeGB := int(volume.RoundUpSize(volSizeBytes, 1024*1024*1024))

	//Volume Name
	volName := req.GetName()
	if len(volName) == 0 {
		volName = uuid.NewUUID().String()
	}

	// Volume Parameters
	glusterVol := req.GetParameters()["glustervol"]
	glusterServer := req.GetParameters()["glusterserver"]
	glusterURL := req.GetParameters()["glusterurl"]
	glusterURLPort := req.GetParameters()["glusterurlport"]
	glusterUser := req.GetParameters()["glusteruser"]
	glusterUserSecret := req.GetParameters()["glusterusersecret"]

	if len(glusterServer) == 0 {
		glusterServer = defaultGlusterServer
	}

	if len(glusterURL) == 0 {
		glusterURL = "http://127.0.0.1:8081"
	} else {
		glusterURL = glusterURL + ":" + glusterURLPort
	}

	cli := gcli.NewClient(glusterURL, glusterUser, glusterUserSecret)
	if cli == nil {
		glog.Errorf("failed to create glusterfs rest client")
		return nil, fmt.Errorf("failed to create glusterfs REST client, REST server authentication failed")
	}

	volumeReq := &gapi.VolumeCreateRequest{Size: volSizeGB}
	volume, err := cli.VolumeCreate(volumeReq)
	if err != nil {
		glog.Errorf("error creating volume %v ", err)
		return nil, fmt.Errorf("error creating volume %v", err)
	}
	glog.V(1).Infof("volume with size: %d and name: %s created", volume.Size, volume.Name)
	glusterVol = volume.Name

	dynamicHostIps, err := getClusterNodes(cli, volume.Cluster)
	if err != nil {
		glog.Errorf("error [%v] when getting cluster nodes for volume %s", err, volume)
		return nil, fmt.Errorf("error [%v] when getting cluster nodes for volume %s", err, volume)
	}
	glog.V(1).Infof("Succesfully created volume '%v'", volName)
	glusterServer = dynamicHostIps[0]

	resp := &csi.CreateVolumeResponse{
		//VolumeInfo: &csi.VolumeInfo{
		Volume: &csi.Volume{
			Id: volume.Id,
			Attributes: map[string]string{
				"glustervol":        glusterVol,
				"glusterserver":     glusterServer,
				"glusterbkpservers": dstrings.Join(dynamicHostIps[0:], ":"),
				"glusterurl":        glusterURL,
				"glusteruser":       glusterUser,
				"glusterusersecret": glusterUserSecret,
			},
		},
	}
	return resp, nil

}

// getClusterNodes() returns the cluster nodes of a given cluster

func getClusterNodes(cli *gcli.Client, cluster string) (dynamicHostIps []string, err error) {
	clusterinfo, err := cli.ClusterInfo(cluster)
	if err != nil {
		glog.Errorf("failed to get cluster details: %v", err)
		return nil, fmt.Errorf("failed to get cluster details: %v", err)
	}

	// For the dynamically provisioned volume, we gather the list of node IPs
	// of the cluster on which provisioned volume belongs to, as there can be multiple
	// clusters.
	for _, node := range clusterinfo.Nodes {
		nodei, err := cli.NodeInfo(string(node))
		if err != nil {
			glog.Errorf(" failed to get hostip: %v", err)
			return nil, fmt.Errorf("failed to get hostip: %v", err)
		}
		ipaddr := dstrings.Join(nodei.NodeAddRequest.Hostnames.Storage, "")
		dynamicHostIps = append(dynamicHostIps, ipaddr)
	}
	glog.V(3).Infof("hostlist :%v", dynamicHostIps)
	if len(dynamicHostIps) == 0 {
		glog.Errorf("no hosts found: %v", err)
		return nil, fmt.Errorf("no hosts found: %v", err)
	}
	return dynamicHostIps, nil
}

func (cs *controllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	if err := cs.Driver.ValidateControllerServiceRequest(req.Version, csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME); err != nil {
		glog.V(3).Infof("invalid delete volume req: %v", req)
		return nil, err
	}
	volumeId := req.VolumeId
	glog.V(4).Infof("deleting volume %s: req: %v", volumeId, req)
	cli := gcli.NewClient(defaultGlusterURL, "", "")
	if cli == nil {
		glog.Errorf("failed to create glusterfs rest client")
		return nil, fmt.Errorf("failed to create glusterfs REST client, REST server authentication failed")
	}

	err := cli.VolumeDelete(volumeId)
	if err != nil {
		glog.Errorf("error deleting volume %v ", err)
		return nil, fmt.Errorf("error deleting volume %v", err)
	}
	glog.V(1).Infof("volume :%s deleted", volumeId)

	return &csi.DeleteVolumeResponse{}, nil
}

func (cs *controllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	for _, cap := range req.VolumeCapabilities {
		if cap.GetAccessMode().GetMode() != csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER {
			return &csi.ValidateVolumeCapabilitiesResponse{Supported: false, Message: ""}, nil
		}
	}
	return &csi.ValidateVolumeCapabilitiesResponse{Supported: true, Message: ""}, nil
}
