package identity

import (
	"context"

	"github.com/gluster/gluster-csi-driver/pkg/command"

	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
)

// Server struct of Glusterfs CSI driver with supported methods of CSI identity server spec.
type Server struct {
	*command.Config
}

// NewServer instantiates a Server
func NewServer(config *command.Config) *Server {
	return &Server{
		Config: config,
	}
}

// GetPluginInfo returns metadata of the plugin
func (is *Server) GetPluginInfo(ctx context.Context, req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	resp := &csi.GetPluginInfoResponse{
		Name:          is.Name,
		VendorVersion: is.Version,
	}
	return resp, nil
}

// GetPluginCapabilities returns available capabilities of the plugin
func (is *Server) GetPluginCapabilities(ctx context.Context, req *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
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
	return resp, nil
}

// Probe returns the health and readiness of the plugin
func (is *Server) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	return &csi.ProbeResponse{}, nil
}
