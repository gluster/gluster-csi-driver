package client

import (
	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
)

// GlusterClient is an interface to clients for different Gluster server types
type GlusterClient interface {
	GetClusterNodes(string) ([]string, error)
	CheckExistingVolume(string, int64) error
	CreateVolume(string, int64, map[string]string) error
	DeleteVolume(string) error
	ListVolumes() ([]*csi.Volume, error)
	CheckExistingSnapshot(string, string) (*csi.Snapshot, error)
	CreateSnapshot(string, string) (*csi.Snapshot, error)
	CloneSnapshot(string, string) error
	DeleteSnapshot(string) error
	ListSnapshots(string, string) ([]*csi.Snapshot, error)
}
