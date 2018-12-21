package command

import (
	"flag"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Config is the driver configuration struct
type Config struct {
	Endpoint string // CSI endpoint
	NodeID   string // CSI node ID
	Name     string // CSI driver name
	Version  string // CSI driver version
	Desc     string // CSI driver description

	RestURL    string // GD2 endpoint
	RestUser   string // GD2 user name who has access to create and delete volume
	RestSecret string // GD2 user password

	CmdName string // Executable name
}

// NewConfig returns config struct to initialize new driver
func NewConfig(cmdName, CSIDriverName, CSIDriverVersion, CSIDriverDesc string) *Config {
	return &Config{
		Name:    CSIDriverName,
		Version: CSIDriverVersion,
		Desc:    CSIDriverDesc,
		CmdName: cmdName,
	}
}

// Driver interface
type Driver interface {
	Run()
}

// Init driver executable function
func Init() {
	// #nosec
	_ = flag.Set("logtostderr", "true")
}

// Run parses flags, sets up cobra Command, and runs the driver
func Run(config *Config, newDriver Driver) {
	// #nosec
	_ = flag.CommandLine.Parse([]string{})

	cmd := &cobra.Command{
		Use:   config.Name,
		Short: config.Desc,
		Run: func(cmd *cobra.Command, args []string) {
			newDriver.Run()
		},
	}

	cmd.Flags().AddGoFlagSet(flag.CommandLine)

	cmd.PersistentFlags().StringVar(&config.NodeID, "nodeid", "", "CSI node id")
	cmd.PersistentFlags().StringVar(&config.Endpoint, "endpoint", "", "CSI endpoint")
	cmd.PersistentFlags().StringVar(&config.RestURL, "resturl", "", "glusterd2 rest endpoint")
	cmd.PersistentFlags().StringVar(&config.RestUser, "username", "glustercli", "glusterd2 user name")
	cmd.PersistentFlags().StringVar(&config.RestSecret, "restsecret", "", "glusterd2 rest user secret")

	if config.NodeID == "" {
		config.NodeID = os.Getenv("CSI_NODE_ID")
	}
	if config.Endpoint == "" {
		config.Endpoint = os.Getenv("CSI_ENDPOINT")
	}
	if config.RestURL == "" {
		config.RestURL = os.Getenv("REST_URL")
	}
	if config.RestUser == "" {
		config.RestUser = os.Getenv("REST_USER")
	}
	if config.RestSecret == "" {
		config.RestSecret = os.Getenv("REST_SECRET")
	}

	if err := cmd.Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%s", err.Error())
		os.Exit(1)
	}
}
