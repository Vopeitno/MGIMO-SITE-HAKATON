package config

import "testing"

func TestSubmissionURL(t *testing.T) {
	for _, link := range []string{DefaultMoodleURL, "https://lms.example.test/mod/assign/view.php?id=42"} {
		if !ValidSubmissionURL(link) {
			t.Fatal("valid link rejected")
		}
	}
	for _, link := range []string{"", "javascript:alert(1)", "http://lms.example.test/", "https://user:password@lms.example.test/", "//lms.example.test/", "https://lms.example.test/\n"} {
		if ValidSubmissionURL(link) {
			t.Fatal("invalid link accepted")
		}
	}
}
