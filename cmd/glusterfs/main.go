package main

import (
	"fmt"
	"os"

	"github.com/gluster/gluster-csi-driver/pkg/command"
	"github.com/gluster/gluster-csi-driver/pkg/glusterfs"
)

// Driver Identifiers
const (
	cmdName          = "glusterfs-csi-driver"
	CSIDriverDesc    = "GlusterFS CSI Driver"
	CSIDriverName    = "org.gluster.glusterfs"
	CSIDriverVersion = "0.0.9"
)

func init() {
	command.Init()
}

func main() {
	var config = command.NewConfig(cmdName, CSIDriverName, CSIDriverVersion, CSIDriverDesc)

	d := glusterfs.New(config, nil)
	if d == nil {
		fmt.Println("Failed to initialize GlusterFS CSI driver")
		os.Exit(1)
	}

	command.Run(config, d)
}
