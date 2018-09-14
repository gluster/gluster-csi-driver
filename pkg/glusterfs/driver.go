package glusterfs

import (
	"os"

	"github.com/gluster/gluster-csi-driver/pkg/command"
	"github.com/gluster/gluster-csi-driver/pkg/controller"

	"github.com/golang/glog"
)

// Driver implements command.Driver
type Driver struct {
	*command.Config
}

// New returns a new Driver
func New(config *command.Config) *Driver {
	gd := &Driver{}

	if config != nil {
		gd.Config = config
	} else {
		glog.Error("failed to initialize GlusterD2 driver: config is nil")
		return nil
	}

	glog.V(1).Infof("%s initialized", gd.Desc)

	return gd
}

// Run runs the driver
func (d *Driver) Run() {
	client, err := NewClient(d.Config)
	if err != nil {
		glog.Errorf("failed to get gd2Client: %v", err)
		os.Exit(1)
	}

	controller.Run(d.Config, client)
}
