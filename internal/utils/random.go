package utils

import (
	"fmt"
	"math/rand"
	"time"

	"retro/internal/config"
	"retro/internal/types"
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
	case types.TimeUnitSeconds:
		return time.Duration(randomVal) * time.Second, nil
	case types.TimeUnitMinutes:
		return time.Duration(randomVal) * time.Minute, nil
	case "":
		return time.Duration(randomVal) * time.Second, nil
	default:
		return 0, fmt.Errorf("unknown delay unit: %s", delayRange.Unit)
	}
}

// init initializes the random number generator seed when the package is loaded
func init() {
	rand.Seed(time.Now().UnixNano())
}
