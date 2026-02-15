package server

import "time"

const asOfLayout = "2006-01-02"

func currentUTCDateString() string {
	return time.Now().UTC().Format(asOfLayout)
}
