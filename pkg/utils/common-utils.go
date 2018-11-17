package utils

import (
	"github.com/gluster/glusterd2/pkg/restclient"
	"github.com/golang/glog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/kubernetes/pkg/util/mount"
	"os"
	"strings"
)

// Common allocation units
const (
	KB int64 = 1000
	MB int64 = 1000 * KB
	GB int64 = 1000 * MB
	TB int64 = 1000 * GB
)

var mounter = mount.New("")

// RoundUpSize calculates how many allocation units are needed to accommodate
// a volume of given size.
// RoundUpSize(1500 * 1000*1000, 1000*1000*1000) returns '2'
// (2 GB is the smallest allocatable volume that can hold 1500MiB)
func RoundUpSize(volumeSizeBytes int64, allocationUnitBytes int64) int64 {
	return (volumeSizeBytes + allocationUnitBytes - 1) / allocationUnitBytes
}

// RoundUpToGB rounds up given quantity upto chunks of GB
func RoundUpToGB(sizeBytes int64) int64 {
	return RoundUpSize(sizeBytes, GB)
}

// MountVolume mounts the given source at the target
func MountVolume(srcPath string, targetPath string, fstype string, mo []string) error {
	notMnt, err := mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			if err = os.MkdirAll(targetPath, 0750); err != nil {
				return status.Error(codes.Internal, err.Error())
			}
			notMnt = true
		} else {
			return status.Error(codes.Internal, err.Error())
		}
	}

	if !notMnt {
		return nil
	}

	err = mounter.Mount(srcPath, targetPath, fstype, mo)
	if err != nil {
		if os.IsPermission(err) {
			return status.Error(codes.PermissionDenied, err.Error())
		}
		if strings.Contains(err.Error(), "invalid argument") {
			return status.Error(codes.InvalidArgument, err.Error())
		}
		return status.Error(codes.Internal, err.Error())
	}
	return nil
}

// GetClusterNodes returns the gluster cluster peer addresses
func GetClusterNodes(client *restclient.Client) (string, []string, error) {
	peers, err := client.Peers()
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

// Config struct fills the parameters of request or user input
type Config struct {
	Endpoint      string // CSI endpoint
	NodeID        string // CSI node ID
	RestURL       string // GD2 endpoint
	RestUser      string // GD2 user name who has access to create and delete volume
	RestSecret    string // GD2 user password
	BlockHostPath string //Gluster volume mount path where the block files will be hosted
}

//NewConfig returns config struct to initialize new driver
func NewConfig() *Config {
	return &Config{}
}
