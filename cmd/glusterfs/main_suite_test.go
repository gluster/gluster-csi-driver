package main_test

import (
	"fmt"
	"testing"

	"github.com/kubernetes-csi/csi-test/v5/pkg/sanity"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/klog/v2"

	"github.com/gluster/gluster-csi-driver/pkg/glusterfs"
)

const (
	endpointSchema = "unix:"
	endpointPath   = "/tmp/glustercsi.socket"
)

func TestGlusterCSIDriver(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Glusterfs Suite")
}

var _ = Describe("GlusterFS CSI driver", func() {
	Context("Normal Config", func() {
		config := &sanity.TestConfig{
			Address:     fmt.Sprintf("%s%s", endpointSchema, endpointPath),
			DialOptions: []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
		}
		options := &glusterfs.DriverOptions{
			Endpoint:   fmt.Sprintf("%s/%s", endpointSchema, endpointPath),
			NodeID:     "1",
			DriverName: "glusterfs",
		}

		var driver *glusterfs.Driver

		BeforeEach(func() {
			driver = glusterfs.NewDriver(options)
			if driver == nil {
				klog.Fatalln("Failed to initialize GlusterFS CSI driver")
			}
			testingMock := true
			driver.Run(options.Endpoint, options.Kubeconfig, testingMock)
		})

		AfterEach(func() {
			driver.Stop()
		})

		Describe("CSI Sanity", func() {
			sanity.GinkgoTest(config)
		})
	})
})
