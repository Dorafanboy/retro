package utils

import (
	"fmt"
	"math/rand"
	"time"

	"retro_template/internal/config"
)

// RandomIntInRange returns a random integer within the range [min, max]
func RandomIntInRange(min, max int) int {
	if min > max {
		min, max = max, min
	}
	if min == max {
		return min
	}
	return rand.Intn(max-min+1) + min
}

// RandomDuration returns a random time.Duration based on the config DelayRange
func RandomDuration(delayRange config.DelayRange) (time.Duration, error) {
	randomVal := RandomIntInRange(delayRange.Min, delayRange.Max)
	switch delayRange.Unit {
	case "seconds":
		return time.Duration(randomVal) * time.Second, nil
	case "minutes":
		return time.Duration(randomVal) * time.Minute, nil
	default:
		return 0, fmt.Errorf("unknown delay unit: %s", delayRange.Unit)
	}
}

// init initializes the random number generator seed when the package is loaded
func init() {
	rand.Seed(time.Now().UnixNano())
}
