package api

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"mgimo/backend/internal/config"
	"testing"
)

func TestAttachmentValidation(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.White)
	var b bytes.Buffer
	if e := png.Encode(&b, img); e != nil {
		t.Fatal(e)
	}
	if ext, e := attachmentType(b.Bytes(), "../../photo.png"); e != nil || ext != ".png" {
		t.Fatal("корректный PNG не принят", e)
	}
	for _, f := range []struct {
		data []byte
		name string
	}{{[]byte("<script>bad</script>"), "photo.png"}, {b.Bytes(), "file.jpg"}, {[]byte("PK broken"), "file.docx"}, {[]byte("%PDF-broken"), "file.pdf"}, {b.Bytes(), "file.exe"}} {
		if _, e := attachmentType(f.data, f.name); e == nil {
			t.Fatal("принят некорректный файл", f.name)
		}
	}
}
func TestAttachmentScannerFailClosed(t *testing.T) {
	t.Setenv("CLAMSCAN_PATH", "/nonexistent/mgimo-scanner")
	if e := attachmentScan(context.Background(), "/nonexistent/document"); e == nil {
		t.Fatal("файл принят без сканера")
	}
}
func TestHandlerRegistersWithoutConflicts(t *testing.T) {
	s, e := New(nil, nil, config.Config{})
	if e != nil {
		t.Fatal(e)
	}
	_ = s.Handler()
}
