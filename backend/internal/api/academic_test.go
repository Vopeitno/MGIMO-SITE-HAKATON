package api

import (
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestAcademicPagination(t *testing.T) {
	for _, tc := range []struct {
		query         string
		limit, offset int
		valid         bool
	}{
		{"", 20, 0, true}, {"limit=100&offset=25", 100, 25, true}, {"limit=1&offset=0", 1, 0, true},
		{"limit=0", 0, 0, false}, {"limit=101", 0, 0, false}, {"offset=-1", 0, 0, false},
		{"limit=", 0, 0, false}, {"offset=abc", 0, 0, false}, {"limit=1&limit=2", 0, 0, false},
		{"offset=0&offset=1", 0, 0, false}, {"offset=999999999999999999999999", 0, 0, false},
	} {
		t.Run(tc.query, func(t *testing.T) {
			q, _ := url.ParseQuery(tc.query)
			l, o, err := academicPagination(q)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
			if tc.valid && (l != tc.limit || o != tc.offset) {
				t.Fatalf("got %d,%d", l, o)
			}
		})
	}
}

func TestAcademicCalendarDates(t *testing.T) {
	for _, tc := range []struct {
		query         string
		period, valid bool
	}{
		{"date=2026-11-18", false, true}, {"from=2026-11-18&to=2026-11-18", true, true},
		{"from=2026-12-31&to=2027-01-01", true, true}, {"date=2024-02-29", false, true},
		{"date=2026-02-29", false, false}, {"date=2026-11-18T00:00:00Z", false, false},
		{"", false, false}, {"date=2026-11-18&date=2026-11-19", false, false},
		{"from=2026-11-19&to=2026-11-18", true, false}, {"from=2026-11-18", true, false},
	} {
		t.Run(tc.query, func(t *testing.T) {
			q, _ := url.ParseQuery(tc.query)
			from, to, err := academicDates(q, tc.period)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
			if !tc.valid {
				return
			}
			_, offset := from.Zone()
			if offset != 3*60*60 {
				t.Fatalf("wrong UTC offset %d", offset)
			}
			if from.Hour() != 0 || to.Before(from) {
				t.Fatal("invalid calendar bounds")
			}
			if to.AddDate(0, 0, 1).Sub(to) != 24*time.Hour {
				t.Fatal("inclusive end day incorrect")
			}
		})
	}
}

func TestAcademicInteger(t *testing.T) {
	for _, v := range []string{"abc", "1 OR 1=1", "2147483648", ""} {
		if _, err := academicInteger(v); err == nil {
			t.Errorf("accepted %q", v)
		}
	}
	for _, v := range []string{"0", "-1", "2147483647"} {
		if _, err := academicInteger(v); err != nil {
			t.Errorf("rejected integer %q", v)
		}
	}
}

func TestAcademicCourseFilter(t *testing.T) {
	for _, tc := range []struct {
		query string
		valid bool
	}{
		{"", true}, {"courseID=55555555-5555-4555-8555-555555555555", true},
		{"courseID=", false}, {"courseID=invalid", false},
		{"courseID=55555555-5555-4555-8555-555555555555&courseID=55555555-5555-4555-8555-555555555555", false},
	} {
		t.Run(tc.query, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/?"+tc.query, nil)
			_, ok := academicCourseFilter(w, r)
			if ok != tc.valid {
				t.Fatalf("valid=%v got=%v", tc.valid, ok)
			}
			if !ok && w.Code != 400 {
				t.Fatalf("status %d", w.Code)
			}
		})
	}
}

func TestAcademicOptionalInteger(t *testing.T) {
	for _, tc := range []struct {
		query string
		valid bool
	}{
		{"", true}, {"groupID=101", true}, {"groupID=", false}, {"groupID=1&groupID=2", false}, {"groupID=1%3BSELECT", false},
	} {
		t.Run(tc.query, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/?"+tc.query, nil)
			_, ok := academicOptionalInteger(w, r, "groupID")
			if ok != tc.valid {
				t.Fatalf("valid=%v got=%v", tc.valid, ok)
			}
			if !ok && w.Code != 400 {
				t.Fatalf("status %d", w.Code)
			}
		})
	}
}

func TestAcademicUUIDPath(t *testing.T) {
	for _, id := range []string{"not-a-uuid", "55555555-5555-4555-8555-555555555555"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		r.SetPathValue("examID", id)
		value, ok := academicPathUUID(w, r, "examID")
		want := academicUUID.MatchString(id)
		if ok != want || (ok && value != id) {
			t.Fatalf("unexpected UUID validation for %q", id)
		}
		if !ok && w.Code != 400 {
			t.Fatalf("status %d", w.Code)
		}
	}
}

func TestAcademicRoles(t *testing.T) {
	for _, role := range []string{"STUDENT", "TEACHER", "DEAN", "SUPPORT", "ADMIN", "", "student"} {
		for _, support := range []bool{false, true} {
			want := role == "STUDENT" || role == "TEACHER" || role == "DEAN" || (role == "SUPPORT" && support)
			if academicRoleAllowed(role, support) != want {
				t.Errorf("role=%q support=%v", role, support)
			}
		}
	}
}
