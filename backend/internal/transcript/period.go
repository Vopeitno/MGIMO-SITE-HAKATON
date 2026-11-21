package transcript

import (
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// StudyYear maps consecutive curriculum semesters to a year of study.
func StudyYear(semester int) int { return (semester + 1) / 2 }
func CurrentYear(enrolled, now time.Time) int {
	moscow := time.FixedZone("МСК", 3*60*60)
	now = now.In(moscow)
	enrolled = enrolled.In(moscow)
	start := now.Year()
	if now.Month() < time.September {
		start--
	}
	year := start - enrolled.Year() + 1
	if year < 1 {
		return 1
	}
	return year
}

// ParseYear allows only one year parameter; zero denotes all years, -1 current.
func ParseYear(raw string) (int, error) {
	q, err := url.ParseQuery(raw)
	if err != nil {
		return 0, err
	}
	if len(q) == 0 {
		return -1, nil
	}
	if len(q) != 1 || len(q["year"]) != 1 {
		return 0, fmt.Errorf("invalid parameters")
	}
	value := q.Get("year")
	if value == "all" {
		return 0, nil
	}
	year, err := strconv.Atoi(value)
	if err != nil || year < 1 || year > 50 || strconv.Itoa(year) != value {
		return 0, fmt.Errorf("invalid year")
	}
	return year, nil
}
func PeriodLabel(year int) string {
	if year == 0 {
		return "Все годы обучения"
	}
	return fmt.Sprintf("%d-й год обучения · семестры %d–%d", year, year*2-1, year*2)
}

// AvailableYears includes missing curriculum years, even without assigned courses.
// The current year extends the list automatically when the academic year changes.
func AvailableYears(current int, curriculum []int) []int {
	last := current
	if last < 1 {
		last = 1
	}
	for _, year := range curriculum {
		if year > last && year <= 50 {
			last = year
		}
	}
	if last > 50 {
		last = 50
	}
	years := make([]int, last)
	for i := range years {
		years[i] = i + 1
	}
	return years
}
