package transcript

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/signintech/gopdf"
)

type ResumeEntry struct {
	Kind        string  `json:"kind"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	URL         *string `json:"url"`
}

type Resume struct {
	DisplayName string        `json:"displayName"`
	Bio         string        `json:"bio"`
	Items       []ResumeEntry `json:"items"`
}

var resumeGroups = [...]struct{ kind, title string }{
	{"project", "Проекты"}, {"certificate", "Сертификаты"}, {"achievement", "Достижения"},
	{"publication", "Публикации"}, {"volunteering", "Волонтёрство"}, {"skill", "Навыки"},
}

var resumeAccent = [3]uint8{87, 72, 176}

func (l *layout) resumePage() {
	l.pdf.AddPage()
	l.pdf.SetFillColor(87, 72, 176)
	l.pdf.RectFromUpperLeftWithStyle(left, 34, width, 3, "F")
	l.text(left, 47, width, 14, "РЕЗЮМЕ СТУДЕНТА", 9, true, resumeAccent, gopdf.Left)
	l.y = 78
}

func (l *layout) resumeSpace(height float64) {
	if l.y+height > 768 {
		l.resumePage()
	}
}

func (l *layout) resumeHeading(title string) {
	l.resumeSpace(58)
	l.rule(l.y)
	l.y += 12
	l.text(left, l.y, width, 22, title, 15, true, resumeAccent, gopdf.Left)
	l.y += 29
}

func (l *layout) resumeText(text string, size, lineHeight float64, bold bool, color [3]uint8) {
	paragraphs := strings.Split(strings.ReplaceAll(strings.TrimSpace(text), "\r\n", "\n"), "\n")
	for _, paragraph := range paragraphs {
		if clean(paragraph) == "" {
			l.y += lineHeight / 2
			continue
		}
		for _, line := range l.lines(paragraph, width, size, bold) {
			l.resumeSpace(lineHeight)
			l.text(left, l.y, width, lineHeight, line, size, bold, color, gopdf.Left)
			l.y += lineHeight
		}
	}
}

// RenderResume exports the same portfolio content and category order as the public view.
func RenderResume(ctx context.Context, data Resume) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	l, err := newLayout()
	if err != nil {
		return nil, err
	}
	l.pdf.SetInfo(gopdf.PdfInfo{Title: "Резюме студента", Author: "Цифровой деканат", CreationDate: time.Now()})
	l.resumePage()
	l.resumeText(data.DisplayName, 27, 34, true, navy)
	l.y += 14
	if data.Bio != "" {
		l.resumeHeading("О себе")
		l.resumeText(data.Bio, 10.5, 16, false, navy)
		l.y += 18
	}
	grouped := make(map[string][]ResumeEntry, len(resumeGroups))
	for _, item := range data.Items {
		grouped[item.Kind] = append(grouped[item.Kind], item)
	}
	for _, group := range resumeGroups {
		if len(grouped[group.kind]) == 0 {
			continue
		}
		l.resumeHeading(group.title)
		for _, item := range grouped[group.kind] {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			l.resumeSpace(36)
			l.resumeText(item.Title, 12.5, 18, true, navy)
			l.y += 3
			l.resumeText(item.Description, 10.5, 16, false, navy)
			if item.URL != nil && *item.URL != "" {
				l.resumeSpace(19)
				l.text(left, l.y, width, 16, "Открыть материал", 10, true, resumeAccent, gopdf.Left)
				l.pdf.AddExternalLink(*item.URL, left, l.y, 100, 16)
				l.y += 19
			}
			l.y += 12
		}
		l.y += 8
	}
	if len(data.Items) == 0 && data.Bio == "" {
		l.resumeText("В портфолио пока нет записей.", 10.5, 16, false, muted)
	}
	pages := l.pdf.GetNumberOfPages()
	for page := 1; page <= pages; page++ {
		if err := l.pdf.SetPage(page); err != nil {
			return nil, err
		}
		l.rule(786)
		l.text(left, 798, width, 14, fmt.Sprintf("Страница %d из %d", page, pages), 8, false, muted, gopdf.Right)
	}
	if l.err != nil {
		return nil, l.err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return l.pdf.GetBytesPdfReturnErr()
}
