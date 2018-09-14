package utils

// Common allocation units
const (
	KB int64 = 1000
	MB int64 = 1000 * KB
	GB int64 = 1000 * MB
	TB int64 = 1000 * GB
)

// RoundUpSize calculates how many allocation units are needed to accommodate
// a volume of given size.
// RoundUpSize(1500 * 1000*1000, 1000*1000*1000) returns '2'
// (2 GB is the smallest allocatable volume that can hold 1500MiB)
func RoundUpSize(volumeSizeBytes int64, allocationUnitBytes int64) int64 {
	return (volumeSizeBytes + allocationUnitBytes - 1) / allocationUnitBytes
}

// RoundUpToGB rounds up given quantity upto chunks of GiB
func RoundUpToGB(sizeBytes int64) int64 {
	return RoundUpSize(sizeBytes, GB)
}

// Config struct fills the parameters of request or user input
type Config struct {
	Endpoint   string // CSI endpoint
	NodeID     string // CSI node ID
	RestURL    string // GD2 endpoint
	RestUser   string // GD2 user name who has access to create and delete volume
	RestSecret string // GD2 user password
}

// CsiDrvParam stores csi driver specific request parameters.
// This struct will be used to gather specific fields of CSI driver:
// For eg. csiDrvName, csiDrvVersion..etc
// and also gather parameters passed from SC which not part of gluster volcreate api.
// glusterCluster - The resturl of gluster cluster
// glusterUser - The gluster username who got access to the APIs.
// glusterUserToken - The password/token of glusterUser to connect to glusterCluster
// glusterVersion - Says the version of the glustercluster running in glusterCluster endpoint.
// compMatrix - map which will be internally defined by the driver to make a compatibility
//              version matrix between CSI driver and gluster cluster. All these fields are optional and can be used if needed.
type CsiDrvParam struct {
	GlusterCluster   string
	GlusterUser      string
	GlusterUserToken string
	GlusterVersion   string
	CsiDrvName       string
	CsiDrvVersion    string
	CompMatrix       map[string]string
}

//NewConfig returns config struct to initialize new driver
func NewConfig() *Config {
	return &Config{}
}
