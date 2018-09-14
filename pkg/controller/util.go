package controller

import (
	"fmt"
	"strconv"
)

func listParseRange(startStr string, end int32) (int32, int32, error) {
	if len(startStr) == 0 {
		startStr = "0"
	}
	s, err := strconv.ParseInt(startStr, 0, 32)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid starting token: %s", startStr)
	}
	start := int32(s)

	if end > 0 {
		end = start + end
	} else {
		end = int32(0)
	}

	return start, end, nil
}

func listGetEnd(length int, start, end int32) (int32, string, error) {
	endStr := ""
	listLen := int32(length)
	if listLen != 0 {
		s := start
		if s < 0 {
			s = -s
		}
		if s >= listLen {
			return 0, "", fmt.Errorf("starting token %d greater than list length", start)
		}
		if end >= listLen || end <= 0 {
			end = listLen
		} else {
			endStr = fmt.Sprintf("%d", end)
		}
	} else {
		end = int32(0)
	}

	return end, endStr, nil
}
