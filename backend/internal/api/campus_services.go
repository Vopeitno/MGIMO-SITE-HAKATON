package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"mgimo/backend/internal/config"
)

var campusServiceRoutes = []studyRoute{
	{"GET", "/campus/benefits", "STUDENT|DEAN", (*Server).campusBenefits},
	{"POST", "/campus/benefits", "DEAN", (*Server).campusBenefitPublish},
	{"PUT", "/campus/benefits/{id}", "DEAN", (*Server).campusBenefitPublish},
	{"DELETE", "/campus/benefits/{id}", "DEAN", (*Server).campusBenefitArchive},
	{"GET", "/campus/lost-items", "STUDENT|SUPPORT", (*Server).campusLostItems},
	{"POST", "/campus/lost-items", "SUPPORT", (*Server).campusLostItemPublish},
	{"DELETE", "/campus/lost-items/{id}", "SUPPORT", (*Server).campusLostItemArchive},
	{"GET", "/campus/lost-reports", "STUDENT|SUPPORT", (*Server).campusLostReports},
	{"POST", "/campus/lost-reports", "STUDENT", (*Server).campusLostReportCreate},
	{"PUT", "/campus/lost-reports/{id}/resolution", "SUPPORT", (*Server).campusLostReportResolve},
	{"GET", "/campus/resources", "SUPPORT", (*Server).campusResources},
	{"POST", "/campus/resources", "SUPPORT", (*Server).campusResourceCreate},
	{"GET", "/campus/resource-slots", "STUDENT|SUPPORT", (*Server).campusSlots},
	{"POST", "/campus/resource-slots", "SUPPORT", (*Server).campusSlotCreate},
	{"DELETE", "/campus/resource-slots/{id}", "SUPPORT", (*Server).campusSlotWithdraw},
	{"PUT", "/campus/resource-slots/{id}/reservation", "STUDENT", (*Server).campusSlotReserve},
	{"DELETE", "/campus/resource-slots/{id}/reservation", "STUDENT", (*Server).campusSlotCancel},
}

func serviceText(value *string, max int) bool {
	*value = strings.TrimSpace(*value)
	return *value != "" && utf8.RuneCountInString(*value) <= max
}

func serviceID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if !resourceUUID(id) {
		fail(w, 400, "BAD_REQUEST", "Некорректный идентификатор")
		return "", false
	}
	return id, true
}

func (s *Server) portalPage(w http.ResponseWriter, r *http.Request, source, order string, args ...any) {
	limit, offset, err := academicPagination(r.URL.Query())
	if err != nil {
		fail(w, 400, "BAD_REQUEST", "Некорректные параметры страницы")
		return
	}
	limitArg := "$" + strconv.Itoa(len(args)+1)
	offsetArg := "$" + strconv.Itoa(len(args)+2)
	s.academicRead(w, r, `WITH records AS (`+source+`), page AS (
		SELECT * FROM records ORDER BY `+order+` LIMIT `+limitArg+` OFFSET `+offsetArg+`
	)
	SELECT jsonb_build_object(
		'items', COALESCE((SELECT jsonb_agg(to_jsonb(page) ORDER BY `+order+`) FROM page), '[]'::jsonb),
		'total', (SELECT count(*) FROM records), 'limit', `+limitArg+`::int, 'offset', `+offsetArg+`::int
	)`, append(args, limit, offset))
}

func (s *Server) campusBenefits(w http.ResponseWriter, r *http.Request) {
	p := current(r)
	s.portalPage(w, r, `SELECT id, title, description, url, published_at AS "publishedAt",
		COALESCE(published_by=$1::uuid, false) AS editable
		FROM campus_benefits WHERE archived_at IS NULL`, `"publishedAt" DESC, id`, p.AccountID)
}

func (s *Server) campusBenefitPublish(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		URL         string `json:"url"`
	}
	if !decode(w, r, &body) {
		return
	}
	body.URL = strings.TrimSpace(body.URL)
	if !serviceText(&body.Title, 120) || !serviceText(&body.Description, 2000) ||
		(body.URL != "" && (len(body.URL) > 1000 || !config.ValidSubmissionURL(body.URL))) {
		fail(w, 422, "VALIDATION_ERROR", "Укажите название, условия льготы и при необходимости ссылку HTTPS")
		return
	}
	account := current(r).AccountID
	if r.Method == http.MethodPost {
		id := uuid.NewString()
		_, err := s.DB.Exec(r.Context(), `INSERT INTO campus_benefits(id,title,description,url,published_by)
			VALUES($1,$2,$3,NULLIF($4,''),$5)`, id, body.Title, body.Description, body.URL, account)
		if err != nil {
			resourceError(w, err)
			return
		}
		write(w, 201, map[string]string{"id": id})
		return
	}
	id, ok := serviceID(w, r)
	if !ok {
		return
	}
	result, err := s.DB.Exec(r.Context(), `UPDATE campus_benefits SET title=$2,description=$3,url=NULLIF($4,'')
		WHERE id=$1 AND published_by=$5 AND archived_at IS NULL`, id, body.Title, body.Description, body.URL, account)
	if err != nil {
		resourceError(w, err)
	} else if result.RowsAffected() == 0 {
		fail(w, 404, "NOT_FOUND", "Льгота недоступна для редактирования")
	} else {
		write(w, 204, nil)
	}
}

func (s *Server) campusBenefitArchive(w http.ResponseWriter, r *http.Request) {
	id, ok := serviceID(w, r)
	if !ok {
		return
	}
	result, err := s.DB.Exec(r.Context(), `UPDATE campus_benefits SET archived_at=COALESCE(archived_at,now())
		WHERE id=$1 AND published_by=$2`, id, current(r).AccountID)
	if err != nil {
		resourceError(w, err)
	} else if result.RowsAffected() == 0 {
		fail(w, 404, "NOT_FOUND", "Льгота недоступна для снятия с публикации")
	} else {
		write(w, 204, nil)
	}
}

func (s *Server) campusLostItems(w http.ResponseWriter, r *http.Request) {
	s.portalPage(w, r, `SELECT id, kind, title, location, published_at AS "publishedAt"
		FROM campus_lost_items WHERE archived_at IS NULL`, `"publishedAt" DESC, id`)
}

func (s *Server) campusLostItemPublish(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Kind     string `json:"kind"`
		Title    string `json:"title"`
		Location string `json:"location"`
	}
	if !decode(w, r, &body) {
		return
	}
	if (body.Kind != "lost" && body.Kind != "found") || !serviceText(&body.Title, 120) || !serviceText(&body.Location, 180) {
		fail(w, 422, "VALIDATION_ERROR", "Выберите тип и укажите название вещи и место")
		return
	}
	id := uuid.NewString()
	_, err := s.DB.Exec(r.Context(), `INSERT INTO campus_lost_items(id,kind,title,location)
		VALUES($1,$2,$3,$4)`, id, body.Kind, body.Title, body.Location)
	if err != nil {
		resourceError(w, err)
		return
	}
	write(w, 201, map[string]string{"id": id})
}

func (s *Server) campusLostItemArchive(w http.ResponseWriter, r *http.Request) {
	id, ok := serviceID(w, r)
	if !ok {
		return
	}
	tag, err := s.DB.Exec(r.Context(), `UPDATE campus_lost_items SET archived_at=COALESCE(archived_at,now()) WHERE id=$1`, id)
	if err != nil {
		resourceError(w, err)
	} else if tag.RowsAffected() == 0 {
		fail(w, 404, "NOT_FOUND", "Вещь не найдена")
	} else {
		write(w, 204, nil)
	}
}

func (s *Server) campusLostReports(w http.ResponseWriter, r *http.Request) {
	p := current(r)
	s.portalPage(w, r, `SELECT report.id, report.kind, report.title, report.description, report.location,
		created_at AS "createdAt", resolved_at AS "resolvedAt", resolution,
		report.item_id AS "itemID", item.title AS "itemTitle",
		CASE WHEN $1::boolean THEN st."studentNumber" END AS "studentNumber",
		CASE WHEN $1::boolean THEN st."studentName" END AS "studentName"
		FROM campus_lost_reports report JOIN students st ON st."studentID"=report.student_id
		LEFT JOIN campus_lost_items item ON item.id=report.item_id
		WHERE $1 OR report.student_id=NULLIF($2,'')::uuid`,
		`"resolvedAt" NULLS FIRST, "createdAt" DESC, id`, p.Role == "SUPPORT", p.StudentID)
}

func (s *Server) campusLostReportCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Kind        string `json:"kind"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Location    string `json:"location"`
		ItemID      string `json:"itemID"`
	}
	if !decode(w, r, &body) {
		return
	}
	if (body.ItemID != "" && !resourceUUID(body.ItemID)) || (body.Kind != "lost" && body.Kind != "found") || !serviceText(&body.Title, 120) ||
		!serviceText(&body.Description, 2000) || !serviceText(&body.Location, 180) {
		fail(w, 422, "VALIDATION_ERROR", "Выберите вид сообщения и заполните название, место и описание")
		return
	}
	id := uuid.NewString()
	tag, err := s.DB.Exec(r.Context(), `WITH item AS (
		SELECT id FROM campus_lost_items WHERE id=NULLIF($7,'')::uuid AND archived_at IS NULL FOR SHARE
	)
	INSERT INTO campus_lost_reports(id,student_id,kind,title,description,location,item_id)
	SELECT $1,$2,$3,$4,$5,$6,(SELECT id FROM item)
	WHERE $7='' OR EXISTS(SELECT 1 FROM item)`,
		id, current(r).StudentID, body.Kind, body.Title, body.Description, body.Location, body.ItemID)
	if err != nil {
		resourceError(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		fail(w, 409, "CONFLICT", "Карточка вещи уже закрыта. Вы можете отправить обычное сообщение.")
		return
	}
	write(w, 201, map[string]string{"id": id})
}

func (s *Server) campusLostReportResolve(w http.ResponseWriter, r *http.Request) {
	id, ok := serviceID(w, r)
	if !ok {
		return
	}
	var body struct {
		Resolution string `json:"resolution"`
	}
	if !decode(w, r, &body) {
		return
	}
	if !serviceText(&body.Resolution, 2000) {
		fail(w, 422, "VALIDATION_ERROR", "Укажите ответ студенту")
		return
	}
	result, err := s.DB.Exec(r.Context(), `UPDATE campus_lost_reports
		SET resolved_at=now(),resolution=$2,resolved_by=$3 WHERE id=$1 AND resolved_at IS NULL`,
		id, body.Resolution, current(r).AccountID)
	if err != nil {
		resourceError(w, err)
	} else if result.RowsAffected() == 0 {
		fail(w, 409, "CONFLICT", "Сообщение уже закрыто или недоступно")
	} else {
		write(w, 204, nil)
	}
}

func (s *Server) campusResources(w http.ResponseWriter, r *http.Request) {
	s.portalPage(w, r, `SELECT id, kind, title, location FROM campus_resources`, "title, location, id")
}

func (s *Server) campusResourceCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Kind     string `json:"kind"`
		Title    string `json:"title"`
		Location string `json:"location"`
	}
	if !decode(w, r, &body) {
		return
	}
	if (body.Kind != "room" && body.Kind != "equipment" && body.Kind != "printing") ||
		!serviceText(&body.Title, 120) || !serviceText(&body.Location, 180) {
		fail(w, 422, "VALIDATION_ERROR", "Выберите тип, укажите название и место выдачи")
		return
	}
	id := uuid.NewString()
	_, err := s.DB.Exec(r.Context(), `INSERT INTO campus_resources(id,kind,title,location) VALUES($1,$2,$3,$4)`,
		id, body.Kind, body.Title, body.Location)
	if err != nil {
		resourceError(w, err)
		return
	}
	write(w, 201, map[string]string{"id": id})
}

func (s *Server) campusSlots(w http.ResponseWriter, r *http.Request) {
	p := current(r)
	s.portalPage(w, r, `SELECT slot.id, res.id AS "resourceID", res.kind, res.title, res.location,
		slot.starts_at AS "startsAt", slot.ends_at AS "endsAt", slot.starts_at>now() AS upcoming,
		slot.reserved_by IS NOT NULL AS reserved,
		COALESCE(slot.reserved_by=NULLIF($2,'')::uuid,false) AS mine,
		CASE WHEN $1::boolean THEN st."studentNumber" END AS "studentNumber",
		CASE WHEN $1::boolean THEN st."studentName" END AS "studentName"
		FROM campus_resource_slots slot JOIN campus_resources res ON res.id=slot.resource_id
		LEFT JOIN students st ON st."studentID"=slot.reserved_by
		WHERE slot.ends_at>now() AND (slot.starts_at>now() OR $1 OR slot.reserved_by=NULLIF($2,'')::uuid)`,
		`"startsAt", id`, p.Role == "SUPPORT", p.StudentID)
}

func (s *Server) campusSlotCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ResourceID string    `json:"resourceID"`
		StartsAt   time.Time `json:"startsAt"`
		EndsAt     time.Time `json:"endsAt"`
	}
	if !decode(w, r, &body) {
		return
	}
	if !resourceUUID(body.ResourceID) || !body.StartsAt.After(time.Now()) || !body.EndsAt.After(body.StartsAt) {
		fail(w, 422, "VALIDATION_ERROR", "Выберите ресурс и будущий интервал")
		return
	}
	ctx := r.Context()
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		resourceError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	// Serialize slot creation for the same resource across server processes.
	var resourceID string
	err = tx.QueryRow(ctx, `SELECT id::text FROM campus_resources WHERE id=$1 FOR UPDATE`, body.ResourceID).Scan(&resourceID)
	if err != nil {
		resourceError(w, err)
		return
	}
	id := uuid.NewString()
	result, err := tx.Exec(ctx, `INSERT INTO campus_resource_slots(id,resource_id,starts_at,ends_at)
		SELECT $1,$2,$3,$4 WHERE $3::timestamptz>now() AND NOT EXISTS (
			SELECT 1 FROM campus_resource_slots WHERE resource_id=$2 AND starts_at<$4 AND ends_at>$3
		)`, id, resourceID, body.StartsAt, body.EndsAt)
	if err != nil {
		resourceError(w, err)
		return
	}
	if result.RowsAffected() == 0 {
		fail(w, 409, "CONFLICT", "Для ресурса уже есть пересекающийся слот или время начала прошло")
		return
	}
	if err = tx.Commit(ctx); err != nil {
		resourceError(w, err)
		return
	}
	write(w, 201, map[string]string{"id": id})
}

func (s *Server) campusSlotWithdraw(w http.ResponseWriter, r *http.Request) {
	id, ok := serviceID(w, r)
	if !ok {
		return
	}
	result, err := s.DB.Exec(r.Context(), `DELETE FROM campus_resource_slots
		WHERE id=$1 AND reserved_by IS NULL AND starts_at>now()`, id)
	if err != nil {
		resourceError(w, err)
	} else if result.RowsAffected() == 0 {
		fail(w, 409, "CONFLICT", "Слот уже забронирован, начался или недоступен")
	} else {
		write(w, 204, nil)
	}
}

func (s *Server) campusSlotReserve(w http.ResponseWriter, r *http.Request) {
	id, ok := serviceID(w, r)
	if !ok {
		return
	}
	// The conditional update locks and rechecks the row if another student books concurrently.
	var reserved bool
	err := s.DB.QueryRow(r.Context(), `UPDATE campus_resource_slots SET reserved_by=$2
		WHERE id=$1 AND starts_at>now() AND (reserved_by IS NULL OR reserved_by=$2)
		RETURNING true`, id, current(r).StudentID).Scan(&reserved)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 409, "CONFLICT", "Слот уже занят, начался или недоступен")
	} else if err != nil {
		resourceError(w, err)
	} else {
		write(w, 204, nil)
	}
}

func (s *Server) campusSlotCancel(w http.ResponseWriter, r *http.Request) {
	id, ok := serviceID(w, r)
	if !ok {
		return
	}
	result, err := s.DB.Exec(r.Context(), `UPDATE campus_resource_slots SET reserved_by=NULL
		WHERE id=$1 AND reserved_by=$2 AND starts_at>now()`, id, current(r).StudentID)
	if err != nil {
		resourceError(w, err)
	} else if result.RowsAffected() == 0 {
		fail(w, 404, "NOT_FOUND", "Активная бронь не найдена или слот уже начался")
	} else {
		write(w, 204, nil)
	}
}
