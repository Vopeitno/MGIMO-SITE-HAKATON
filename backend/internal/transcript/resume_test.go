package transcript

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestResumeManyEntries(t *testing.T) {
	url := "https://example.test/project"
	d := Resume{DisplayName: "Мария Примерова", Bio: "Первый абзац.\n\nВторой абзац."}
	for i := range 60 {
		group := resumeGroups[i%len(resumeGroups)]
		d.Items = append(d.Items, ResumeEntry{group.kind, fmt.Sprintf("Работа %d", i+1),
			strings.Repeat("Длинное описание достижения. ", 8), &url})
	}
	d.Items = append(d.Items, ResumeEntry{"skill", "Конец документа", strings.Repeat("Сверхдлинноеслово", 120), nil})
	pdf, err := RenderResume(context.Background(), d)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) || !bytes.Contains(pdf, []byte("/URI (https://example.test/project)")) ||
		!regexp.MustCompile(`/Count\s+([2-9]|[1-9][0-9]+)\b`).Match(pdf) {
		t.Fatal("Ожидались страницы PDF и действующие ссылки")
	}
	if path := os.Getenv("MGIMO_RESUME_TEST_PDF"); path != "" {
		if err := os.WriteFile(path, pdf, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
