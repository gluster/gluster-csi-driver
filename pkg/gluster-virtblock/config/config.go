package config

// MntInfo struct has information about the block host volume mount path
type MntInfo struct {
	MntPath  string
	RefCount int
}

// Config struct fills the parameters of request or user input
type Config struct {
	Endpoint      string              // CSI endpoint
	NodeID        string              // CSI node ID
	RestURL       string              // GD2 endpoint
	RestUser      string              // GD2 user name who has access to create and delete volume
	RestSecret    string              // GD2 user password
	RestTimeout   int                 // GD2 rest client timeout
	Mounts        map[string]*MntInfo // List of volumes and mount paths
	MntPathPrefix string              // Path under which gluster block host volumes will be mounted
}

//NewConfig returns config struct to initialize new driver
func NewConfig() *Config {
	var conf Config
	conf.Mounts = make(map[string]*MntInfo)
	return &conf
}
