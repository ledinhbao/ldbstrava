package ldbstrava

import (
	"time"

	"github.com/jinzhu/gorm"
)

// Activity contains data for strava's activity. Currently, it presents only distance, moving time, elapsed time and type.
type Activity struct {
	gorm.Model
	StravaID       uint
	Distance       float64
	MovingTime     uint
	ElapsedTime    uint
	Type           string
	Name           string
	StartDate      time.Time
	StartLocalDate time.Time
	AthleteID      uint
}
