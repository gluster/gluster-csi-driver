package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"

	gfd "github.com/gluster/gluster-csi-driver/pkg/glusterfs"
	"k8s.io/component-base/metrics/legacyregistry"
	"k8s.io/klog/v2"

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
			exportMetrics(options)
			handle(options)
		},
	}

	cmd.Flags().AddGoFlagSet(flag.CommandLine)
	cmd.PersistentFlags().StringVar(&options.NodeID, "nodeid", "", "CSI node id")
	_ = cmd.MarkPersistentFlagRequired("nodeid")
	cmd.PersistentFlags().StringVar(&options.Endpoint, "endpoint", "", "CSI endpoint to connect to")
	cmd.PersistentFlags().StringVar(&options.DriverName, "driver-name", "", "CSI driver name")
	cmd.PersistentFlags().StringVar(&options.Kubeconfig, "kubeconfig", "", "Absolute path to the kubeconfig file. Required only when running out of cluster.")
	cmd.PersistentFlags().StringVar(&options.MetricsAddress, "metrics-address", "", "metrics address")

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
		klog.Fatalln("Failed to initialize GlusterFS CSI driver")
	}
	testingMock := false
	d.Run(options.Endpoint, options.Kubeconfig, testingMock)
}

func exportMetrics(options *gfd.DriverOptions) {
	if options.MetricsAddress == "" {
		return
	}
	l, err := net.Listen("tcp", options.MetricsAddress)
	if err != nil {
		klog.Warningf("failed to get listener for metrics endpoint: %v", err)
		return
	}
	serve(context.Background(), l, serveMetrics)
}

func serve(ctx context.Context, l net.Listener, serveFunc func(net.Listener) error) {
	path := l.Addr().String()
	klog.V(2).Infof("set up prometheus server on %v", path)
	go func() {
		defer l.Close()
		if err := serveFunc(l); err != nil {
			klog.Fatalf("serve failure(%v), address(%v)", err, path)
		}
	}()
}

func serveMetrics(l net.Listener) error {
	m := http.NewServeMux()
	m.Handle("/metrics", legacyregistry.Handler())
	return trapClosedConnErr(http.Serve(l, m))
}

func trapClosedConnErr(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "use of closed network connection") {
		return nil
	}
	return err
}
