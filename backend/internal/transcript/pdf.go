// Package transcript renders private academic statements without external services.
package transcript

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/signintech/gopdf"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
)

type Course struct {
	Title           string
	Semester, Hours int
	Score           *int
	Credit          *bool
}
type Data struct {
	Name, Number, Group, Faculty, Direction string
	Period                                  string
	GeneratedAt                             time.Time
	Courses                                 []Course
}

const MaxCourses = 2000
const left = 44.0
const width = 507.28

var navy = [3]uint8{25, 43, 66}
var muted = [3]uint8{99, 113, 130}

type layout struct {
	pdf gopdf.GoPdf
	err error
	y   float64
}

func clean(s string) string {
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }), " ")
}
func (l *layout) text(x, y, w, h float64, s string, size float64, bold bool, color [3]uint8, align int) {
	if l.err != nil {
		return
	}
	font := "regular"
	if bold {
		font = "bold"
	}
	l.err = l.pdf.SetFont(font, "", size)
	if l.err != nil {
		return
	}
	l.pdf.SetTextColor(color[0], color[1], color[2])
	l.pdf.SetXY(x, y)
	l.err = l.pdf.CellWithOption(&gopdf.Rect{W: w, H: h}, s, gopdf.CellOption{Align: align | gopdf.Middle})
}
func (l *layout) lines(s string, w, size float64, bold bool) []string {
	if l.err != nil {
		return nil
	}
	font := "regular"
	if bold {
		font = "bold"
	}
	l.err = l.pdf.SetFont(font, "", size)
	if l.err != nil {
		return nil
	}
	var lines []string
	lines, l.err = l.pdf.SplitTextWithWordWrap(clean(s), w)
	return lines
}
func (l *layout) rule(y float64) {
	l.pdf.SetStrokeColor(216, 224, 232)
	l.pdf.SetLineWidth(.6)
	l.pdf.Line(left, y, left+width, y)
}
func (l *layout) page(first bool, d Data) {
	l.pdf.AddPage()
	l.text(left, 32, width, 15, "ЦИФРОВОЙ ДЕКАНАТ", 10, true, navy, gopdf.Left)
	l.text(left, 49, width, 12, "Личный кабинет студента · Академические результаты", 8, false, muted, gopdf.Left)
	l.rule(73)
	if first {
		l.text(left, 89, width, 32, "Выписка об успеваемости", 24, true, navy, gopdf.Left)
		l.y = 137
		for _, item := range [][2]string{{"СТУДЕНТ", d.Name}, {"НОМЕР СТУДЕНТА / ГРУППА", d.Number + " / " + d.Group}, {"ФАКУЛЬТЕТ", d.Faculty}, {"НАПРАВЛЕНИЕ", d.Direction}} {
			l.text(left, l.y, width, 12, item[0], 7.5, true, muted, gopdf.Left)
			l.y += 15
			value := clean(item[1])
			if value == "" {
				value = "—"
			}
			for _, line := range l.lines(value, width, 10.5, false) {
				l.text(left, l.y, width, 14, line, 10.5, false, navy, gopdf.Left)
				l.y += 14
			}
			l.y += 10
		}
		if d.Period != "" {
			l.text(left, l.y, width, 14, d.Period, 9, true, navy, gopdf.Left)
			l.y += 22
		}
		zone := time.FixedZone("МСК", 3*60*60)
		l.text(left, l.y, width, 14, "Сформировано: "+d.GeneratedAt.In(zone).Format("02.01.2006, 15:04")+" МСК", 8.5, false, muted, gopdf.Left)
		l.y += 32
	} else {
		l.text(left, 86, width, 20, "Выписка об успеваемости · продолжение", 14, true, navy, gopdf.Left)
		l.y = 122
	}
}

var columns = []float64{25, 242.28, 47, 45, 58, 90}

func (l *layout) header() {
	l.pdf.SetFillColor(navy[0], navy[1], navy[2])
	l.pdf.RectFromUpperLeftWithStyle(left, l.y, width, 30, "F")
	x := left
	for i, s := range []string{"№", "Дисциплина", "Семестр", "Часы", "Баллы¹", "Зачёт"} {
		align := gopdf.Center
		if i == 1 {
			align = gopdf.Left
		}
		l.text(x+7, l.y, columns[i]-14, 30, s, 8, true, [3]uint8{255, 255, 255}, align)
		x += columns[i]
	}
	l.y += 30
}

func newLayout() (*layout, error) {
	l := &layout{}
	l.pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	if err := l.pdf.AddTTFFontData("regular", goregular.TTF); err != nil {
		return nil, err
	}
	if err := l.pdf.AddTTFFontData("bold", gobold.TTF); err != nil {
		return nil, err
	}
	return l, nil
}

// Render uses embedded Cyrillic fonts and returns an entirely self-contained A4 PDF.
func Render(ctx context.Context, d Data) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(d.Courses) > MaxCourses {
		return nil, fmt.Errorf("too many courses")
	}
	l, err := newLayout()
	if err != nil {
		return nil, err
	}
	l.pdf.SetInfo(gopdf.PdfInfo{Title: "Выписка об успеваемости", Author: "Цифровой деканат", CreationDate: d.GeneratedAt})
	l.page(true, d)
	l.header()
	sum, count, credited := 0, 0, 0
	for i, c := range d.Courses {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		lines := l.lines(c.Title, columns[1]-16, 9.5, false)
		h := float64(len(lines))*13 + 18
		if h < 38 {
			h = 38
		}
		if h > 570 {
			return nil, fmt.Errorf("course title too long")
		}
		if l.y+h > 730 || (i == len(d.Courses)-1 && l.y+h+128 > 762) {
			l.page(false, d)
			l.header()
		}
		if i%2 == 0 {
			l.pdf.SetFillColor(244, 247, 250)
			l.pdf.RectFromUpperLeftWithStyle(left, l.y, width, h, "F")
		}
		values := []string{fmt.Sprint(i + 1), "", fmt.Sprint(c.Semester), fmt.Sprint(c.Hours), "—", "Не выставлен"}
		if c.Score != nil {
			values[4] = fmt.Sprint(*c.Score)
			sum += *c.Score
			count++
		}
		if c.Credit != nil {
			if *c.Credit {
				values[5] = "Зачтено"
				credited++
			} else {
				values[5] = "Не зачтено"
			}
		}
		x := left
		for col, v := range values {
			if col == 1 {
				for j, line := range lines {
					l.text(x+8, l.y+9+float64(j)*13, columns[col]-16, 13, line, 9.5, false, navy, gopdf.Left)
				}
			} else {
				l.text(x+5, l.y, columns[col]-10, h, v, 8.5, col == 4, navy, gopdf.Center)
			}
			x += columns[col]
		}
		l.y += h
		l.rule(l.y)
	}
	if len(d.Courses) == 0 {
		l.text(left, l.y+10, width, 24, "Доступных дисциплин пока нет", 10, false, muted, gopdf.Center)
		l.y += 48
	}
	if l.y+108 > 762 {
		l.page(false, d)
	}
	l.y += 20
	l.rule(l.y)
	l.y += 12
	average := "—"
	if count > 0 {
		average = strings.ReplaceAll(fmt.Sprintf("%.1f", float64(sum)/float64(count)), ".", ",")
	}
	for i, v := range [][2]string{{"ДИСЦИПЛИН", fmt.Sprint(len(d.Courses))}, {"ЗАЧТЕНО", fmt.Sprint(credited)}, {"СРЕДНИЙ БАЛЛ¹", average}} {
		x := left + float64(i)*width/3
		l.text(x, l.y, width/3, 12, v[0], 7.5, true, muted, gopdf.Left)
		l.text(x, l.y+17, width/3, 26, v[1], 20, true, navy, gopdf.Left)
	}
	l.text(left, l.y+55, width, 14, "¹ Баллы по шкале 0–100. Прочерк означает, что оценка не выставлена.", 8, false, muted, gopdf.Left)
	pages := l.pdf.GetNumberOfPages()
	for page := 1; page <= pages; page++ {
		if err := l.pdf.SetPage(page); err != nil {
			return nil, err
		}
		l.rule(786)
		l.text(left, 796, width, 12, "Учебный прототип. Не является официальной справкой университета.", 7.5, false, muted, gopdf.Left)
		l.text(left, 814, width, 12, fmt.Sprintf("Страница %d из %d", page, pages), 7.5, false, muted, gopdf.Right)
	}
	if l.err != nil {
		return nil, l.err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return l.pdf.GetBytesPdfReturnErr()
}
