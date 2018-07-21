package main

import (
	"flag"
	"fmt"
	"os"

	gfd "github.com/gluster/gluster-csi-driver/pkg/glusterfs"
	"github.com/gluster/gluster-csi-driver/pkg/glusterfs/utils"

	"github.com/spf13/cobra"
)

func init() {
	flag.Set("logtostderr", "true")
}

func main() {
	flag.CommandLine.Parse([]string{})
	var config = utils.NewConfig()

	cmd := &cobra.Command{
		Use:   "glusterfs-csi-driver",
		Short: "GlusterFS CSI driver",
		Run: func(cmd *cobra.Command, args []string) {
			handle(config)
		},
	}

	cmd.Flags().AddGoFlagSet(flag.CommandLine)

	cmd.PersistentFlags().StringVar(&config.NodeID, "nodeid", "", "CSI node id")
	cmd.MarkPersistentFlagRequired("nodeid")

	cmd.PersistentFlags().StringVar(&config.Endpoint, "endpoint", "", "CSI endpoint")

	cmd.PersistentFlags().StringVar(&config.RestURL, "resturl", "", "glusterd2 rest endpoint")

	cmd.PersistentFlags().StringVar(&config.RestUser, "username", "glustercli", "glusterd2 user name")

	cmd.PersistentFlags().StringVar(&config.RestSecret, "restsecret", "", "glusterd2 rest user secret")

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%s", err.Error())
		os.Exit(1)
	}
}

func handle(config *utils.Config) {
	ep := os.Getenv("CSI_ENDPOINT")
	if config.Endpoint == "" {
		config.Endpoint = ep
	}
	d := gfd.New(config)
	d.Run()
}
