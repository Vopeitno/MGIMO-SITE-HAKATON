package transcript

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRender(t *testing.T) {
	zero := 0
	yes, no := true, false
	d := Data{Name: "Петров Александр Сергеевич", Number: "20260001", Group: "101", Faculty: "Международные отношения", Direction: "Международные отношения и внешняя политика", GeneratedAt: time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC), Courses: []Course{{"История международных отношений", 1, 72, &zero, &no}, {"Иностранный язык (английский)", 1, 144, nil, nil}, {"Основы международного права", 1, 108, ptr(94), &yes}}}
	for _, kind := range []string{"preview", "empty", "multipage"} {
		t.Run(kind, func(t *testing.T) {
			sample := d
			if kind == "empty" {
				sample.Courses = nil
			}
			if kind == "multipage" {
				sample.Name = "Александрова-Константинопольская Екатерина Александровна"
				sample.Courses = nil
				for i := 0; i < 65; i++ {
					sample.Courses = append(sample.Courses, Course{Title: fmt.Sprintf("Дисциплина %02d. История дипломатических отношений и международного сотрудничества в современном мире", i+1), Semester: i/10 + 1, Hours: 72, Score: ptr(i), Credit: &yes})
				}
			}
			pdf, err := Render(context.Background(), sample)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.HasPrefix(pdf, []byte("%PDF-")) || !bytes.Contains(pdf, []byte("%%EOF")) {
				t.Fatal("invalid PDF")
			}
			if dir := os.Getenv("TRANSCRIPT_ARTIFACT_DIR"); dir != "" {
				if err := os.WriteFile(filepath.Join(dir, kind+".pdf"), pdf, 0600); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Render(ctx, d); err == nil {
		t.Fatal("cancelled request rendered")
	}
	d.Courses = make([]Course, MaxCourses+1)
	if _, err := Render(context.Background(), d); err == nil {
		t.Fatal("oversized transcript rendered")
	}
}
func ptr(v int) *int { return &v }
