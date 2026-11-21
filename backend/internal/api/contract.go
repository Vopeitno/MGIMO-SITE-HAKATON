package api

import (
	"net/http"
	"sort"
	"strings"
)

func init() {
	sort.SliceStable(contractRules, func(i, j int) bool {
		return strings.Count(contractRules[i].path, "{") < strings.Count(contractRules[j].path, "{")
	})
}

type contractRule struct {
	method, path string
	roles        []string
}

var contractRules = []contractRule{
	{"GET", "/api/v1/MGIMO/messages/deanery-contacts", []string{"STUDENT"}},
	{"PATCH", "/api/v1/MGIMO/academic/deadlines/{deadlineID}/submission-link", []string{"TEACHER"}},
	{"GET", "/api/v1/MGIMO/academic/transcript/periods", []string{"STUDENT"}},
	{"GET", "/api/v1/MGIMO/academic/transcript.pdf", []string{"STUDENT"}},
	{"POST", "/api/v1/MGIMO/auth/login", []string{"PUBLIC"}},
	{"POST", "/api/v1/MGIMO/auth/refresh", []string{"PUBLIC"}},
	{"POST", "/api/v1/MGIMO/auth/logout", []string{"STUDENT", "TEACHER", "SUPPORT", "DEAN", "ADMIN"}},
	{"GET", "/api/v1/MGIMO/auth/me", []string{"STUDENT", "TEACHER", "SUPPORT", "DEAN", "ADMIN"}},
	{"GET", "/api/v1/MGIMO/profile/me/photo", []string{"STUDENT"}},
	{"PUT", "/api/v1/MGIMO/profile/me/photo", []string{"STUDENT"}},
	{"GET", "/api/v1/MGIMO/profile/me/qr", []string{"STUDENT"}},
	{"GET", "/api/v1/MGIMO/profile/me", []string{"STUDENT", "TEACHER", "SUPPORT", "DEAN", "ADMIN"}},
	{"GET", "/api/v1/MGIMO/schedule", []string{"STUDENT", "TEACHER", "SUPPORT", "DEAN"}},
	{"GET", "/api/v1/MGIMO/schedule/period", []string{"STUDENT", "TEACHER", "SUPPORT", "DEAN"}},
	{"GET", "/api/v1/MGIMO/schedule/changes", []string{"STUDENT", "TEACHER", "SUPPORT", "DEAN"}},
	{"GET", "/api/v1/MGIMO/lessons/{lessonID}", []string{"STUDENT", "TEACHER", "SUPPORT", "DEAN"}},
	{"PATCH", "/api/v1/MGIMO/lessons/{lessonID}", []string{"SUPPORT"}},
	{"GET", "/api/v1/MGIMO/academic/subjects", []string{"STUDENT", "TEACHER", "DEAN"}},
	{"GET", "/api/v1/MGIMO/academic/grades", []string{"STUDENT", "TEACHER", "DEAN"}},
	{"POST", "/api/v1/MGIMO/academic/grades", []string{"TEACHER"}},
	{"PATCH", "/api/v1/MGIMO/academic/grades/{courseGraduate}", []string{"TEACHER"}},
	{"GET", "/api/v1/MGIMO/academic/deadlines", []string{"STUDENT", "TEACHER", "DEAN"}},
	{"GET", "/api/v1/MGIMO/academic/subjects/{courseID}/results", []string{"STUDENT", "TEACHER", "DEAN"}},
	{"POST", "/api/v1/MGIMO/requests", []string{"STUDENT", "TEACHER", "DEAN"}},
	{"GET", "/api/v1/MGIMO/requests", []string{"STUDENT", "TEACHER", "DEAN"}},
	{"GET", "/api/v1/MGIMO/requests/{appoinmentID}", []string{"STUDENT", "TEACHER", "DEAN", "SUPPORT"}},
	{"GET", "/api/v1/MGIMO/requests/{appoinmentID}/status", []string{"STUDENT", "TEACHER", "DEAN", "SUPPORT"}},
	{"PATCH", "/api/v1/MGIMO/requests/{appoinmentID}/status", []string{"SUPPORT"}},
	{"GET", "/api/v1/MGIMO/requests/{appoinmentID}/history", []string{"STUDENT", "TEACHER", "DEAN", "SUPPORT"}},
	{"POST", "/api/v1/MGIMO/requests/{appoinmentID}/attachment", []string{"STUDENT", "TEACHER", "DEAN", "SUPPORT"}},
	{"GET", "/api/v1/MGIMO/requests/{appoinmentID}/attachment", []string{"STUDENT", "TEACHER", "DEAN", "SUPPORT"}},
	{"GET", "/api/v1/MGIMO/requests/assigned", []string{"SUPPORT"}},
	{"POST", "/api/v1/MGIMO/requests/{appoinmentID}/transfer", []string{"SUPPORT"}},
	{"GET", "/api/v1/MGIMO/notifications", []string{"STUDENT", "TEACHER", "DEAN", "SUPPORT", "ADMIN"}},
	{"GET", "/api/v1/MGIMO/notifications/{notificationID}", []string{"STUDENT", "TEACHER", "DEAN", "SUPPORT", "ADMIN"}},
	{"GET", "/api/v1/MGIMO/courses", []string{"STUDENT", "TEACHER", "SUPPORT", "DEAN"}},
	{"POST", "/api/v1/MGIMO/courses", []string{"DEAN", "SUPPORT"}},
	{"GET", "/api/v1/MGIMO/courses/{courseID}", []string{"STUDENT", "TEACHER", "SUPPORT", "DEAN"}},
	{"PATCH", "/api/v1/MGIMO/courses/{courseID}", []string{"DEAN", "SUPPORT"}},
	{"GET", "/api/v1/MGIMO/news", []string{"STUDENT", "TEACHER", "SUPPORT", "DEAN", "ADMIN"}},
	{"POST", "/api/v1/MGIMO/news", []string{"DEAN", "SUPPORT"}},
	{"GET", "/api/v1/MGIMO/faculty/students", []string{"DEAN"}},
	{"GET", "/api/v1/MGIMO/faculty/teachers", []string{"DEAN"}},
	{"GET", "/api/v1/MGIMO/buildings", []string{"STUDENT", "TEACHER", "SUPPORT", "DEAN"}},
	{"GET", "/api/v1/MGIMO/dashboard", []string{"STUDENT", "TEACHER", "DEAN", "SUPPORT"}},
	{"GET", "/api/v1/MGIMO/academic/exams", []string{"STUDENT", "TEACHER", "DEAN"}},
	{"GET", "/api/v1/MGIMO/academic/exams/{examID}", []string{"STUDENT", "TEACHER", "DEAN"}},
	{"GET", "/api/v1/MGIMO/academic/exams/{examID}/results", []string{"STUDENT", "TEACHER", "DEAN"}},
	{"POST", "/api/v1/MGIMO/academic/exams/{examID}/results", []string{"TEACHER"}},
	{"PATCH", "/api/v1/MGIMO/academic/exam-results/{examResultID}", []string{"TEACHER"}},
	{"GET", "/api/v1/MGIMO/sports", []string{"STUDENT"}},
	{"GET", "/api/v1/MGIMO/sports/{sportID}", []string{"STUDENT"}},
	{"GET", "/api/v1/MGIMO/sports/registrations/me", []string{"STUDENT"}},
	{"POST", "/api/v1/MGIMO/sports/{sportID}/registrations", []string{"STUDENT"}},
	{"DELETE", "/api/v1/MGIMO/sports/{sportID}/registrations/me", []string{"STUDENT"}},
	{"GET", "/api/v1/MGIMO/requests/types", []string{"STUDENT", "TEACHER", "DEAN"}},
	{"GET", "/api/v1/MGIMO/requests/drafts", []string{"STUDENT", "TEACHER", "DEAN"}},
	{"POST", "/api/v1/MGIMO/requests/drafts", []string{"STUDENT", "TEACHER", "DEAN"}},
	{"GET", "/api/v1/MGIMO/requests/drafts/{appointmentDraftID}", []string{"STUDENT", "TEACHER", "DEAN"}},
	{"PATCH", "/api/v1/MGIMO/requests/drafts/{appointmentDraftID}", []string{"STUDENT", "TEACHER", "DEAN"}},
	{"DELETE", "/api/v1/MGIMO/requests/drafts/{appointmentDraftID}", []string{"STUDENT", "TEACHER", "DEAN"}},
	{"POST", "/api/v1/MGIMO/requests/drafts/{appointmentDraftID}/submit", []string{"STUDENT", "TEACHER", "DEAN"}},
	{"GET", "/api/v1/MGIMO/requests/unassigned", []string{"SUPPORT"}},
	{"POST", "/api/v1/MGIMO/requests/{appoinmentID}/claim", []string{"SUPPORT"}},
	{"GET", "/api/v1/MGIMO/requests/{appoinmentID}/assignments", []string{"SUPPORT", "DEAN"}},
	{"PATCH", "/api/v1/MGIMO/notifications/{notificationID}/read", []string{"STUDENT", "TEACHER", "SUPPORT", "DEAN", "ADMIN"}},
	{"GET", "/api/v1/MGIMO/news/{newsID}", []string{"STUDENT", "TEACHER", "SUPPORT", "DEAN", "ADMIN"}},
	{"PATCH", "/api/v1/MGIMO/news/{newsID}", []string{"SUPPORT", "DEAN"}},
	{"POST", "/api/v1/MGIMO/admin/accounts", []string{"ADMIN"}},
	{"PATCH", "/api/v1/MGIMO/admin/accounts/{accountID}", []string{"ADMIN"}},
	{"POST", "/api/v1/MGIMO/requests/drafts/{appointmentDraftID}/attachment", []string{"STUDENT", "TEACHER", "DEAN"}},
	{"GET", "/api/v1/MGIMO/requests/drafts/{appointmentDraftID}/attachment", []string{"STUDENT", "TEACHER", "DEAN"}},
}

func contractRoleAllowed(r *http.Request, role string) bool {
	for _, rule := range contractRules {
		if rule.method != r.Method || !contractPathMatch(rule.path, r.URL.Path) {
			continue
		}
		for _, allowed := range rule.roles {
			if role == allowed {
				return true
			}
		}
		return false
	}
	// Дополнительный согласованный контракт личных диалогов проверяет участников сам.
	for _, route := range studyAndSettingsRoutes {
		if r.Method == route.method && contractPathMatch(prefix+route.path, r.URL.Path) {
			return strings.Contains("|"+route.role+"|", "|"+role+"|")
		}
	}
	return strings.HasPrefix(r.URL.Path, prefix+"/messages/")
}
func contractPathMatch(pattern, path string) bool {
	a, b := strings.Split(strings.Trim(pattern, "/"), "/"), strings.Split(strings.Trim(path, "/"), "/")
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if strings.HasPrefix(v, "{") && strings.HasSuffix(v, "}") {
			continue
		}
		if v != b[i] {
			return false
		}
	}
	return true
}
