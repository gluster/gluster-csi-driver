package main

import (
	"flag"
	"fmt"
	"os"

	gfd "github.com/gluster/gluster-csi-driver/pkg/glusterfs"
	"github.com/gluster/gluster-csi-driver/pkg/glusterfs/config"
	"github.com/spf13/cobra"
)

func init() {
	// #nosec
	_ = flag.Set("logtostderr", "true")
}

func main() {
	// #nosec
	_ = flag.CommandLine.Parse([]string{})
	var config = config.NewConfig()

	cmd := &cobra.Command{
		Use:   "glusterfs-csi-driver",
		Short: "GlusterFS CSI driver",
		Run: func(cmd *cobra.Command, args []string) {
			handle(config)
		},
	}

	cmd.Flags().AddGoFlagSet(flag.CommandLine)

	cmd.PersistentFlags().StringVar(&config.NodeID, "nodeid", "", "CSI node id")
	// #nosec
	_ = cmd.MarkPersistentFlagRequired("nodeid")

	cmd.PersistentFlags().StringVar(&config.Endpoint, "endpoint", "", "CSI endpoint")

	cmd.PersistentFlags().StringVar(&config.RestURL, "resturl", "", "glusterd2 rest endpoint")

	cmd.PersistentFlags().StringVar(&config.RestUser, "username", "glustercli", "glusterd2 user name")

	cmd.PersistentFlags().StringVar(&config.RestSecret, "restsecret", "", "glusterd2 rest user secret")

	cmd.PersistentFlags().IntVar(&config.RestTimeout, "resttimeout", 30, "glusterd2 rest client timeout")

	if err := cmd.Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%s", err.Error())
		os.Exit(1)
	}
}

func handle(config *config.Config) {
	if config.Endpoint == "" {
		config.Endpoint = os.Getenv("CSI_ENDPOINT")
	}
	d := gfd.New(config)
	if d == nil {
		fmt.Println("Failed to initialize GlusterFS CSI driver")
		os.Exit(1)
	}
	d.Run()
}
