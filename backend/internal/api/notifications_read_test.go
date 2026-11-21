package api

import (
	"context"
	"net/http"
	"testing"
)

func TestNotificationReadIsolation(t *testing.T) {
	f := requestTestFixture(t)
	f.mux = http.NewServeMux()
	(&Server{DB: f.pool}).registerResources(f.mux)
	ctx := context.Background()
	own, other := requestTestID(601), requestTestID(602)
	if _, err := f.pool.Exec(ctx, `INSERT INTO notifications("notificationID","studentID","notificationTitle","notificationMessage","notificationCategory","notificationCreatedAt") VALUES($1,$2,'Своё','Текст','news',now()),($3,$4,'Чужое','Текст','news',now())`, own, f.student.StudentID, other, f.other.StudentID); err != nil {
		t.Fatal(err)
	}
	status, result := f.call(f.student, "GET", "/notifications", "")
	requestTestStatus(t, 200, status, result)
	items := result["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["notificationID"] != own {
		t.Fatal("notification privacy")
	}
	status, result = f.call(f.student, "PATCH", "/notifications/"+own+"/read", "")
	requestTestStatus(t, 200, status, result)
	read := result["notificationReadAt"]
	if read == nil {
		t.Fatal("missing timestamp")
	}
	status, result = f.call(f.student, "PATCH", "/notifications/"+own+"/read", "")
	requestTestStatus(t, 200, status, result)
	if result["notificationReadAt"] != read {
		t.Fatal("read timestamp overwritten")
	}
	status, result = f.call(f.student, "PATCH", "/notifications/"+other+"/read", "")
	requestTestStatus(t, 404, status, result)
}
