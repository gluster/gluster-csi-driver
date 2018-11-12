package client

import (
	"strconv"

	"github.com/golang/glog"
)

// Common allocation units
const (
	KB int64 = 1000
	MB int64 = 1000 * KB
	GB int64 = 1000 * MB
	TB int64 = 1000 * GB

	KiB int64 = 1024
	MiB int64 = 1024 * KiB
	GiB int64 = 1024 * MiB
)

// RoundUpSize calculates how many allocation units are needed to accommodate
// a volume of given size.
// RoundUpSize(1500 * 1000*1000, 1000*1000*1000) returns '2'
// (2 GB is the smallest allocatable volume that can hold 1500MiB)
func RoundUpSize(volumeSizeBytes int64, allocationUnitBytes int64) int64 {
	return (volumeSizeBytes + allocationUnitBytes - 1) / allocationUnitBytes
}

// RoundUpToGB rounds up given quantity upto chunks of GB
func RoundUpToGB(sizeBytes int64) int64 {
	return RoundUpSize(sizeBytes, GB)
}

// RoundUpToMiB rounds up given quantity upto chunks of MiB
func RoundUpToMiB(sizeBytes int64) int64 {
	return RoundUpSize(sizeBytes, MiB)
}

// RoundUpToGiB rounds up given quantity upto chunks of GiB
func RoundUpToGiB(sizeBytes int64) int64 {
	return RoundUpSize(sizeBytes, GiB)
}

// SetPointerIfEmpty returns a new parameter if the old parameter is empty
func SetPointerIfEmpty(old, new interface{}) interface{} {
	if old == nil {
		return new
	}
	return old
}

// SetStringIfEmpty returns a new parameter if the old parameter is empty
func SetStringIfEmpty(old, new string) string {
	if len(old) == 0 {
		return new
	}
	return old
}

// ParseIntWithDefault parses a string into an int, using a default for an
// empty or illegal string
func ParseIntWithDefault(new string, defInt int) int {
	newInt := defInt

	if len(new) != 0 {
		parsedInt, err := strconv.Atoi(new)
		if err != nil {
			glog.Errorf("bad int string [%s], using default [%s]", new, defInt)
		} else {
			newInt = parsedInt
		}
	}

	return newInt
}

// ParseBoolWithDefault parses a string into a bool, using a default for an
// empty or illegal string
func ParseBoolWithDefault(new string, defBool bool) bool {
	newBool := defBool

	if len(new) != 0 {
		parsedBool, err := strconv.ParseBool(new)
		if err != nil {
			glog.Errorf("bad bool string [%s], using default [%s]", new, defBool)
		} else {
			newBool = parsedBool
		}
	}

	return newBool
}

// ParseFloatWithDefault parses a string into an float32, using a default for an
// empty or illegal string
func ParseFloatWithDefault(new string, defFloat float32) float32 {
	newFloat := defFloat

	if len(new) != 0 {
		parsedFloat, err := strconv.ParseFloat(new, 32)
		if err != nil {
			glog.Errorf("bad float string [%s], using default [%s]", new, defFloat)
		} else {
			newFloat = float32(parsedFloat)
		}
	}

	return newFloat
}
