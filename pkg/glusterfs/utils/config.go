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

//NewConfig returns config struct to initialize new driver
func NewConfig() *Config {
	return &Config{}
}
