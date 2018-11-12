package main

import (
	"fmt"
	"os"

	"github.com/gluster/gluster-csi-driver/pkg/command"
	"github.com/gluster/gluster-csi-driver/pkg/node"
)

// Driver Identifiers
const (
	cmdName          = "glusterfs-node-driver"
	CSIDriverDesc    = "GlusterFS (glusterd2) CSI Node Driver"
	CSIDriverName    = "org.gluster.glusterfs"
	CSIDriverVersion = "0.0.9"
)

func init() {
	command.Init()
}

func main() {
	var config = command.NewConfig(cmdName, CSIDriverName, CSIDriverVersion, CSIDriverDesc)

	d := node.New(config)
	if d == nil {
		fmt.Println("Failed to initialize GlusterFS CSI driver")
		os.Exit(1)
	}

	cmd := command.InitCommand(config, d)

	command.Run(config, cmd)
}
