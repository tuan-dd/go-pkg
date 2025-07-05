package typeCustom

import (
	"fmt"
	"time"
)

type DurationHMS string

func (d *DurationHMS) ToInt() int64 {
	if d == nil || len(*d) == 0 {
		return 0
	}

	if parsedTime, err := time.Parse(time.DateOnly, string(*d)); err == nil {
		duration := time.Duration(parsedTime.Hour())*time.Hour +
			time.Duration(parsedTime.Minute())*time.Minute +
			time.Duration(parsedTime.Second())*time.Second
		return int64(duration.Seconds())
	}
	return 0
}

func NewDurationHMS(d int64) *DurationHMS {
	if d < 0 {
		return nil
	}

	hours := d / 3600
	minutes := (d % 3600) / 60
	seconds := d % 60

	duration := DurationHMS(fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds))
	return &duration
}
