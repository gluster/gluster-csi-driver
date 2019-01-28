package config

// Config struct fills the parameters of request or user input
type Config struct {
	Endpoint      string // CSI endpoint
	NodeID        string // CSI node ID
	RestURL       string // GD2 endpoint
	RestUser      string // GD2 user name who has access to create and delete volume
	RestSecret    string // GD2 user password
	RestTimeout   int    // GD2 rest client timeout
	BlockHostPath string //Gluster volume mount path where the block files will be hosted
}

//NewConfig returns config struct to initialize new driver
func NewConfig() *Config {
	return &Config{}
}
