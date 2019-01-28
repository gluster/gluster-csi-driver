package glustervirtblock

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/glog"
)

// IdentityServer struct of Glusterfs CSI driver with supported methods of CSI identity server spec.
type IdentityServer struct {
	*GfDriver
}

// GetPluginInfo returns metadata of the plugin
func (is *IdentityServer) GetPluginInfo(ctx context.Context, req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	resp := &csi.GetPluginInfoResponse{
		Name:          glusterBlockCSIDriverName,
		VendorVersion: glusterBlockCSIDriverVersion,
	}
	glog.V(1).Infof("plugininfo response: %+v", resp)
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
	glog.V(1).Infof("plugin capability response: %+v", resp)
	return resp, nil
}

// Probe returns the health and readiness of the plugin
func (is *IdentityServer) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	return &csi.ProbeResponse{}, nil
}
