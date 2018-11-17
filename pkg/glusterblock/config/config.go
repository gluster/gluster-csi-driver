package config

// Config struct fills the parameters of request or user input
type Config struct {
	Endpoint      string            // CSI endpoint
	NodeID        string            // CSI node ID
	RestURL       string            // GD2 endpoint
	RestUser      string            // GD2 user name who has access to create and delete volume
	RestSecret    string            // GD2 user password
	Inited        bool              // indicates whether the initial block hosting volumes are mounted
	Mounts        map[string]string // List of volumes and mount paths
	MntPathPrefix string            // Path under which gluster block host volumes will be mounted
	BlockHostSize int64             // Block host volume size
	GlusterServer string            // Peer address of the volfile server
	BkpServers    []string          // backup volfile servers
}

//NewConfig returns config struct to initialize new driver
func NewConfig() *Config {
	var conf Config
	conf.Mounts = make(map[string]string)
	return &conf
}
