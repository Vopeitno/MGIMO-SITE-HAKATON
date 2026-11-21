package api

import (
	"encoding/json"
	"time"
)

type sportPeriod struct {
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

var sportPeriods = [...]sportPeriod{
	{"08:10", "09:40"}, {"10:10", "11:40"}, {"12:10", "13:40"},
	{"14:10", "15:40"}, {"16:10", "17:40"}, {"17:50", "19:20"},
}

var sportPeriodsJSON = func() string {
	raw, err := json.Marshal(sportPeriods)
	if err != nil {
		panic(err)
	}
	return string(raw)
}()

func validSportPeriod(start, end string) bool {
	for _, period := range sportPeriods {
		if period.StartTime == start && period.EndTime == end {
			return true
		}
	}
	return false
}

const sportWeekLoad = `SELECT count(*) FROM (
	SELECT a."sportID" FROM sport_attendance a
	WHERE a."studentID"=$1::uuid AND a.present AND a."weekStart"=$2::text::date
	UNION
	SELECT sp."sportID" FROM sport_registrations r JOIN sports sp USING ("sportID")
	WHERE r."studentID"=$1::uuid AND r."registrationStatus"='enrolled'
	AND date_trunc('week',sp."sportStartAt" AT TIME ZONE 'Europe/Moscow')::date=$2::text::date
`

func sportWeek(date time.Time) string {
	local := date.In(time.FixedZone("MSK", 3*60*60))
	daysFromMonday := (int(local.Weekday()) + 6) % 7
	return local.AddDate(0, 0, -daysFromMonday).Format("2006-01-02")
}
