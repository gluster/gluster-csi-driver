package main

import (
	"flag"
	"fmt"
	"os"

	gfd "github.com/gluster/gluster-csi-driver/pkg/glusterfs"

	"github.com/spf13/cobra"
)

func init() {
	_ = flag.Set("logtostderr", "true")
}

func main() {
	_ = flag.CommandLine.Parse([]string{})
	var options = &gfd.DriverOptions{}

	cmd := &cobra.Command{
		Use:   "glusterfs-csi-driver",
		Short: "GlusterFS CSI driver",
		Run: func(cmd *cobra.Command, args []string) {
			handle(options)
		},
	}

	cmd.Flags().AddGoFlagSet(flag.CommandLine)

	cmd.PersistentFlags().StringVar(&options.NodeID, "nodeid", "", "CSI node id")
	_ = cmd.MarkPersistentFlagRequired("nodeid")

	cmd.PersistentFlags().StringVar(&options.DriverName, "driver-name", "", "CSI driver name")

	cmd.PersistentFlags().StringVar(&options.Kubeconfig, "kubeconfig", "", "Absolute path to the kubeconfig file. Required only when running out of cluster.")

	if err := cmd.Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%s", err.Error())
		os.Exit(1)
	}
}

func handle(options *gfd.DriverOptions) {
	if options.Endpoint == "" {
		options.Endpoint = os.Getenv("CSI_ENDPOINT")
	}
	d := gfd.NewDriver(options)
	if d == nil {
		fmt.Println("Failed to initialize GlusterFS CSI driver")
		os.Exit(1)
	}
	d.Run(options.Endpoint, options.Kubeconfig, false)
}
