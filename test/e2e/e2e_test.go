package e2e

import (
	"flag"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/kubernetes/test/e2e/framework"
)

var viperConfig = flag.String("viper-config", "", "The name of a viper config file (https://github.com/spf13/viper#what-is-viper). All e2e command line parameters can also be configured in such a file. May contain a path and may or may not contain the file suffix. The default is to look for an optional file with `e2e` as base name. If a file is specified explicitly, it must be present.")

func init() {
	// Register framework flags, then handle flags and Viper config.
	framework.HandleFlags()
	/*
		if err := viperconfig.ViperizeFlags(*viperConfig, "e2e"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	*/
	framework.AfterReadingAllFlags(&framework.TestContext)
}

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2e Suite")
}
