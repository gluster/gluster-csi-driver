package node

import (
	"github.com/gluster/gluster-csi-driver/pkg/command"

	"github.com/golang/glog"
)

// Driver implements command.Driver
type Driver struct {
	*command.Config
}

// New returns a new Driver
func New(config *command.Config) *Driver {
	nd := &Driver{}

	if config != nil {
		nd.Config = config
	} else {
		glog.Error("failed to initialize GlusterFS node driver: config is nil")
		return nil
	}

	glog.V(1).Infof("%s initialized", nd.Desc)

	return nd
}

// Run runs the driver
func (d *Driver) Run() {
	Run(d.Config)
}
