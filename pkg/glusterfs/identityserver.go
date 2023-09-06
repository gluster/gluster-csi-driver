package glusterfs

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/protobuf/ptypes/wrappers"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

// IdentityServer struct of Glusterfs CSI driver with supported methods of CSI
// identity server spec.
type IdentityServer struct {
	*Driver
}

// NewIdentityServer initialize an identity server for GlusterFS CSI driver.
func NewIdentityServer(g *Driver) *IdentityServer {
	return &IdentityServer{
		Driver: g,
	}
}

// GetPluginInfo returns metadata of the plugin
func (ids *IdentityServer) GetPluginInfo(ctx context.Context, req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	if ids.Driver.name == "" {
		return nil, status.Error(codes.Unavailable, "Driver name not configured")
	}

	if ids.Driver.version == "" {
		return nil, status.Error(codes.Unavailable, "Driver is missing version")
	}

	resp := &csi.GetPluginInfoResponse{
		Name:          glusterfsCSIDriverName,
		VendorVersion: glusterfsCSIDriverVersion,
	}
	klog.V(1).Infof("plugininfo response: %+v", resp)
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
	klog.V(1).Infof("plugin capability response: %+v", resp)
	return resp, nil
}

// Probe returns the health and readiness of the plugin
func (is *IdentityServer) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	return &csi.ProbeResponse{Ready: &wrappers.BoolValue{Value: true}}, nil
}
