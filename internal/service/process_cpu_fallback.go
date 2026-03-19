//go:build !darwin && !linux

package service

import "time"

func currentProcessCPUTime() (time.Duration, bool) {
	return 0, false
}
