package glusterfs

import (
	"context"

	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	"github.com/golang/glog"
)

type IdentityServer struct {
	*GfDriver
}

// GetPluginInfo returns metadata of the plugin
func (is *IdentityServer) GetPluginInfo(ctx context.Context, req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	resp := &csi.GetPluginInfoResponse{
		Name:          driverName,
		VendorVersion: vendorVersion,
	}
	glog.V(1).Infof("%+v, plugininfo response: %+v", resp)
	return resp, nil
}

// GetPluginCapabilities returns available capabilities of the plugin
func (is *IdentityServer) GetPluginCapabilities(ctx context.Context, req *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	resp := &csi.GetPluginCapabilitiesResponse{
		Capabilities: []*csi.PluginCapability{
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
					},
				},
			},
		},
	}
	glog.V(1).Infof("Response: %+v", resp)
	return resp, nil
}

// Probe returns the health and readiness of the plugin
func (is *IdentityServer) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	return &csi.ProbeResponse{}, nil
}
