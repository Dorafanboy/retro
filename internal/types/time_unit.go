package types

// TimeUnit определяет единицы измерения времени для задержек.
// EN: TimeUnit defines the units for time delays.
type TimeUnit string

const (
	// TimeUnitSeconds представляет секунды.
	// EN: TimeUnitSeconds represents seconds.
	TimeUnitSeconds TimeUnit = "seconds"
	// TimeUnitMinutes представляет минуты.
	// EN: TimeUnitMinutes represents minutes.
	TimeUnitMinutes TimeUnit = "minutes"
)
