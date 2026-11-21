package api

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"mgimo/backend/internal/config"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestRequestRealAntivirusUpload(t *testing.T) {
	if os.Getenv("REAL_CLAMAV_TEST") != "true" {
		t.Skip("requires real ClamAV signatures")
	}
	f := requestTestFixture(t)
	f.mux = http.NewServeMux()
	(&Server{DB: f.pool, Config: config.Config{UploadDir: t.TempDir()}}).registerRequests(f.mux)
	status, v := f.call(f.student, "POST", "/requests/drafts", fmt.Sprintf(`{"appointmentTypeID":%q,"appointmentTitle":"Справка","appointmentDescription":"Описание"}`, f.typeID))
	requestTestStatus(t, 201, status, v)
	id := requestString(v["appointmentDraftID"])
	var imageData bytes.Buffer
	if err := png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, err := form.CreateFormFile("file", "photo.png")
	if err != nil {
		t.Fatal(err)
	}
	part.Write(imageData.Bytes())
	form.Close()
	r := httptest.NewRequest("POST", "/api/v1/MGIMO/requests/drafts/"+id+"/attachment", &body)
	r.Header.Set("Content-Type", form.FormDataContentType())
	r = r.WithContext(context.WithValue(r.Context(), principalKey{}, f.student))
	w := httptest.NewRecorder()
	f.mux.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("real scanner upload: %d %s", w.Code, w.Body.String())
	}
	status, v = f.call(f.student, "POST", "/requests/drafts/"+id+"/submit", "")
	requestTestStatus(t, 201, status, v)
	if v["fileUrlAttachment"] == nil {
		t.Fatal("attachment was lost during submission")
	}
	r = httptest.NewRequest("GET", "/api/v1/MGIMO/requests/"+id+"/attachment", nil)
	r = r.WithContext(context.WithValue(r.Context(), principalKey{}, f.student))
	w = httptest.NewRecorder()
	f.mux.ServeHTTP(w, r)
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), imageData.Bytes()) {
		t.Fatal("downloaded file mismatch")
	}
	r = httptest.NewRequest("GET", "/api/v1/MGIMO/requests/"+id+"/attachment", nil)
	r = r.WithContext(context.WithValue(r.Context(), principalKey{}, f.other))
	w = httptest.NewRecorder()
	f.mux.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("other student downloaded attachment")
	}
}
