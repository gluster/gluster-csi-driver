package utils

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/gluster/glusterd2/pkg/restclient"
	"github.com/golang/glog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/kubernetes/pkg/util/mount"
)

// Common allocation units
const (
	KB int64 = 1000
	MB int64 = 1000 * KB
	GB int64 = 1000 * MB
	TB int64 = 1000 * GB

	minReplicaCount = 1
	maxReplicaCount = 10
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

// ValidateThinArbiter validates the thin arbiter volume parameters
func ValidateThinArbiter(req *csi.CreateVolumeRequest) error {
	if _, ok := req.Parameters["arbiterPath"]; ok {
		rc, ok := req.Parameters["replicas"]
		if ok {
			count, err := strconv.ParseInt(rc, 10, 32)
			if err != nil {
				return err
			}
			if count != 2 {
				return errors.New("thin arbiter can only be enabled for replica count 2")
			}
		}
	} else {
		return errors.New("thin arbiterPath not specified")
	}
	return nil
}

// ParseVolumeParamInt validates replicaCount
func ParseVolumeParamInt(key, valueString string) (int, error) {
	errPrefix := fmt.Sprintf("invalid value for parameter '%s'", key)
	count, err := strconv.Atoi(valueString)
	if err != nil {
		return 0, fmt.Errorf("%s, value '%s' must be an integer between %d and %d", errPrefix, valueString, minReplicaCount, maxReplicaCount)
	}

	if count < minReplicaCount {
		return 0, fmt.Errorf("%s, value '%s' must be >= %v", errPrefix, valueString, minReplicaCount)
	}
	if count > maxReplicaCount {
		return 0, fmt.Errorf("%s, value '%s' must be <= %v", errPrefix, valueString, maxReplicaCount)
	}

	return count, nil
}
