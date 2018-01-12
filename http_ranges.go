package main

import (
	"errors"
	"strconv"
	"strings"
)

func parseRange(s string) (int64, int64, error) {
	const b = "bytes="
	if !strings.HasPrefix(s, b) {
		return 0, 0, errors.New("invalid range")
	}

	var rangeStart int64
	var rangeEnd int64
	var err error

	for _, ra := range strings.Split(s[len(b):], ",") {
		ra = strings.TrimSpace(ra)
		if ra == "" {
			continue
		}
		i := strings.Index(ra, "-")
		if i < 0 {
			return 0, 0, errors.New("invalid range")
		}
		start, end := strings.TrimSpace(ra[:i]), strings.TrimSpace(ra[i+1:])
		if start == "" {
			rangeEnd, err = strconv.ParseInt(end, 10, 64)
			if err != nil {
				return 0, 0, errors.New("invalid range")
			}
		} else {

			rangeStart, err = strconv.ParseInt(start, 10, 64)
			if err != nil || i < 0 {
				return 0, 0, errors.New("invalid range")
			}

			rangeEnd, err = strconv.ParseInt(end, 10, 64)
			if err != nil || rangeStart > rangeEnd {
				return 0, 0, errors.New("invalid range")
			}
		}
	}

	return rangeStart, rangeEnd, nil
}
