package transcript

import (
	"reflect"
	"testing"
	"time"
)

func TestPeriod(t *testing.T) {
	for sem, want := range map[int]int{1: 1, 2: 1, 3: 2, 4: 2, 7: 4, 8: 4} {
		if StudyYear(sem) != want {
			t.Fatal(sem)
		}
	}
	enrolled := time.Date(2024, 8, 20, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		now  string
		want int
	}{{"2026-08-31T20:59:59Z", 2}, {"2026-08-31T21:00:00Z", 3}, {"2026-01-15T00:00:00Z", 2}, {"2023-10-01T00:00:00Z", 1}} {
		now, _ := time.Parse(time.RFC3339, tc.now)
		if got := CurrentYear(enrolled, now); got != tc.want {
			t.Fatalf("%s: %d", tc.now, got)
		}
	}
	for raw, want := range map[string]int{"": -1, "year=1": 1, "year=4": 4, "year=all": 0} {
		got, err := ParseYear(raw)
		if err != nil || got != want {
			t.Fatal(raw, got, err)
		}
	}
	for _, raw := range []string{"year=0", "year=-1", "year=51", "year=01", "year=2&year=all", "studentID=other", "year=1&studentID=other", "year=", "year=x", "year=%zz", "format=pdf"} {
		if _, err := ParseYear(raw); err == nil {
			t.Fatal("accepted", raw)
		}
	}
	if PeriodLabel(0) != "Все годы обучения" || PeriodLabel(2) != "2-й год обучения · семестры 3–4" {
		t.Fatal("period labels")
	}
}

func TestAvailableYears(t *testing.T) {
	for _, tc := range []struct {
		current          int
		curriculum, want []int
	}{
		{3, []int{1}, []int{1, 2, 3}},
		{3, []int{1, 3}, []int{1, 2, 3}},
		{4, []int{1, 3}, []int{1, 2, 3, 4}},
		{1, []int{4}, []int{1, 2, 3, 4}},
		{1, nil, []int{1}},
	} {
		if got := AvailableYears(tc.current, tc.curriculum); !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("current=%d curriculum=%v: got %v want %v", tc.current, tc.curriculum, got, tc.want)
		}
	}
	enrolled := time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)
	before := time.Date(2027, 8, 31, 20, 59, 59, 0, time.UTC)
	after := before.Add(time.Second)
	if len(AvailableYears(CurrentYear(enrolled, before), []int{1})) != 3 || len(AvailableYears(CurrentYear(enrolled, after), []int{1})) != 4 {
		t.Fatal("new academic year did not add fourth option")
	}
}
