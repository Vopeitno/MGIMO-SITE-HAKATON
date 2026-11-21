package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/signintech/gopdf"
	"github.com/skip2/go-qrcode"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
)

const activityTicketPrefix = "MGIMO-ACTIVITY:"
const activityHostEligible = `EXISTS(SELECT 1 FROM "facultyTeacher" ft JOIN teachers t USING ("teacherID")
 WHERE ft."facultyID"=$2::uuid AND t."teacherID"=$10::uuid AND ` + studyTeacherActive + `)`
const activityHostScope = `($1='SUPPORT' OR ($1='TEACHER' AND EXISTS(
 SELECT 1 FROM campus_event_hosts h JOIN teachers t USING ("teacherID")
 WHERE h."eventID"=e."eventID" AND h."teacherID"=NULLIF($2,'')::uuid AND ` + studyTeacherActive + `)))`

func (s *Server) activityAssignHost(w http.ResponseWriter, r *http.Request) {
	event, ok := academicPathUUID(w, r, "eventID")
	if !ok {
		return
	}
	var in struct{ TeacherID string }
	if !decode(w, r, &in) {
		return
	}
	if !resourceUUID(in.TeacherID) {
		fail(w, 422, "VALIDATION_ERROR", "Выберите преподавателя")
		return
	}
	studyWrite(s, w, r, func(ctx context.Context, tx pgx.Tx) (any, error) {
		var locked int
		err := tx.QueryRow(ctx, `SELECT 1 FROM campus_events e JOIN (`+studyDeanFaculties+`) f USING ("facultyID")
           WHERE e."eventID"=$2::uuid FOR UPDATE OF e`, current(r).EmployerID, event).Scan(&locked)
		if err != nil {
			return nil, err
		}
		return studyRead(ctx, tx, `INSERT INTO campus_event_hosts ("eventID","teacherID")
          SELECT e."eventID",t."teacherID" FROM campus_events e JOIN "facultyTeacher" ft USING ("facultyID")
          JOIN teachers t USING ("teacherID") WHERE e."eventID"=$1::uuid AND t."teacherID"=$2::uuid AND `+studyTeacherActive+`
          ON CONFLICT ("eventID") DO UPDATE SET "teacherID"=excluded."teacherID"
          RETURNING jsonb_build_object('teacherID',"teacherID")`, event, in.TeacherID)
	})
}

func (s *Server) activityCheckInList(w http.ResponseWriter, r *http.Request) {
	p := current(r)
	s.academicRead(w, r, `SELECT COALESCE(jsonb_agg(`+eventJSON+` ORDER BY e."startAt" DESC),'[]')
        FROM campus_events e JOIN buildings b USING ("buildingID") WHERE `+activityHostScope, []any{p.Role, p.TeacherID})
}

func (s *Server) activityScan(w http.ResponseWriter, r *http.Request) {
	event, ok := academicPathUUID(w, r, "eventID")
	if !ok {
		return
	}
	var in struct{ Code string }
	if !decode(w, r, &in) {
		return
	}
	id, found := strings.CutPrefix(strings.TrimSpace(in.Code), activityTicketPrefix)
	if !found || !resourceUUID(id) {
		fail(w, 422, "VALIDATION_ERROR", "Это не билет на мероприятие")
		return
	}
	p := current(r)
	studyWrite(s, w, r, func(ctx context.Context, tx pgx.Tx) (any, error) {
		var started, finished bool
		err := tx.QueryRow(ctx, `SELECT e."startAt"<=now(),e."endAt"<now() FROM campus_events e
         WHERE e."eventID"=$3::uuid AND `+activityHostScope+` FOR SHARE OF e`, p.Role, p.TeacherID, event).Scan(&started, &finished)
		if err != nil {
			return nil, err
		}
		if !started || finished {
			return nil, studyConflict("QR-код можно отметить только во время мероприятия")
		}
		var student, name string
		var already bool
		err = tx.QueryRow(ctx, `SELECT r."studentID"::text,st."studentName",r."attendedAt" IS NOT NULL
           FROM campus_event_registrations r JOIN students st USING ("studentID")
           WHERE r."eventID"=$1::uuid AND r."ticketID"=$2::uuid AND r."canceledAt" IS NULL
           FOR UPDATE OF r`, event, id).Scan(&student, &name, &already)
		if err != nil {
			return nil, err
		}
		if !already {
			_, err = tx.Exec(ctx, `UPDATE campus_event_registrations SET "attendedAt"=now(),"checkedByAccountID"=$3::uuid,"checkMethod"='qr'
             WHERE "eventID"=$1::uuid AND "studentID"=$2::uuid`, event, student, p.AccountID)
		}
		return map[string]any{"name": name, "already": already, "present": true}, err
	})
}

type campusTicket struct {
	ID, Title, Building string
	Start, End          time.Time
}

func (s *Server) readActivityTicket(w http.ResponseWriter, r *http.Request) (campusTicket, bool) {
	var t campusTicket
	event, ok := academicPathUUID(w, r, "eventID")
	if !ok {
		return t, false
	}
	err := s.DB.QueryRow(r.Context(), `SELECT r."ticketID"::text,e.title,b.building,e."startAt",e."endAt"
      FROM campus_event_registrations r JOIN campus_events e USING ("eventID") JOIN buildings b USING ("buildingID")
      WHERE r."eventID"=$1::uuid AND r."studentID"=$2::uuid AND r."canceledAt" IS NULL`, event, current(r).StudentID).
		Scan(&t.ID, &t.Title, &t.Building, &t.Start, &t.End)
	if err != nil {
		resourceError(w, err)
		return t, false
	}
	return t, true
}

func (s *Server) activityTicketQR(w http.ResponseWriter, r *http.Request) {
	t, ok := s.readActivityTicket(w, r)
	if !ok {
		return
	}
	png, err := qrcode.Encode(activityTicketPrefix+t.ID, qrcode.Medium, 320)
	if err != nil {
		resourceError(w, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	_, _ = w.Write(png)
}

func (s *Server) activityTicketPDF(w http.ResponseWriter, r *http.Request) {
	t, ok := s.readActivityTicket(w, r)
	if !ok {
		return
	}
	pdf, err := renderActivityTicket(t)
	if err != nil {
		resourceError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="campus-ticket.pdf"`)
	_, _ = w.Write(pdf)
}

func renderActivityTicket(t campusTicket) ([]byte, error) {
	qr, err := qrcode.Encode(activityTicketPrefix+t.ID, qrcode.Medium, 400)
	if err != nil {
		return nil, err
	}
	holder, err := gopdf.ImageHolderByBytes(qr)
	if err != nil {
		return nil, err
	}
	doc := &gopdf.GoPdf{}
	doc.Start(gopdf.Config{PageSize: *gopdf.PageSizeA5})
	if err = doc.AddTTFFontData("regular", goregular.TTF); err != nil {
		return nil, err
	}
	if err = doc.AddTTFFontData("bold", gobold.TTF); err != nil {
		return nil, err
	}
	doc.AddPage()
	doc.SetTextColor(42, 31, 71)
	if err = doc.SetFont("bold", "", 22); err != nil {
		return nil, err
	}
	doc.SetXY(32, 32)
	if err = doc.Cell(nil, "Билет на мероприятие"); err != nil {
		return nil, err
	}
	if err = doc.SetFont("regular", "", 13); err != nil {
		return nil, err
	}
	loc := time.FixedZone("MSK", 3*60*60)
	doc.SetXY(32, 78)
	when := t.Start.In(loc).Format("02.01.2006 15:04") + " — " + t.End.In(loc).Format("02.01.2006 15:04") + " МСК"
	if err = doc.MultiCell(&gopdf.Rect{W: 350, H: 76}, strings.Join(strings.Fields(t.Title), " ")); err != nil {
		return nil, err
	}
	doc.SetXY(32, 151)
	if err = doc.Cell(nil, when); err != nil {
		return nil, err
	}
	doc.SetXY(32, 171)
	if err = doc.MultiCell(&gopdf.Rect{W: 350, H: 40}, strings.Join(strings.Fields(t.Building), " ")); err != nil {
		return nil, err
	}
	if err = doc.ImageByHolder(holder, 90, 210, &gopdf.Rect{W: 240, H: 240}); err != nil {
		return nil, err
	}
	doc.SetXY(32, 483)
	if err = doc.MultiCell(&gopdf.Rect{W: 350, H: 52}, "Покажите этот код ответственному на мероприятии. Код привязан к вашей записи."); err != nil {
		return nil, err
	}
	return doc.GetBytesPdfReturnErr()
}
