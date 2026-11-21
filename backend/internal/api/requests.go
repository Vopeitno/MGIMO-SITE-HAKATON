package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// All projections are explicit: joined personnel records must never be serialized.
const requestColumns = `"appoinmentID", "appointmentTypeID", "studentID", "teacherID", "createdByEmployerID", "subjectStudentID", "employerID", "buildingID", "appointmentTitle", "appointmentDescription", "fileUrlAttachment", "appointmentStatus", created_at, updated_at`
const requestDraftColumns = `"appointmentDraftID", "studentID", "teacherID", "createdByEmployerID", "appointmentTypeID", "appointmentTitle", "appointmentDescription", "fileUrlAttachment", "draftStatus", "createdAt", "updatedAt"`
const requestHistoryColumns = `"appointmentHistoryID", "appointmentID", "appointmentStatus", "changedAt", reason, "changedByAccountID"`
const requestAssignmentColumns = `"assignmentID", "appointmentID", "oldEmployerID", "newEmployerID", "changedByAccountID", "changedAt"`
const requestTypeColumns = `"appointmentTypeID", "appointmentTypeTitle", "appointmentTypeDescription", "isActive"`

type requestDB interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}
type requestProblem struct {
	status        int
	code, message string
}

func (e *requestProblem) Error() string { return e.message }
func requestBad(status int, message string) error {
	codes := map[int]string{400: "BAD_REQUEST", 401: "UNAUTHORIZED", 403: "FORBIDDEN", 404: "NOT_FOUND", 409: "CONFLICT", 422: "VALIDATION_ERROR"}
	return &requestProblem{status, codes[status], message}
}
func requestError(w http.ResponseWriter, err error) {
	var p *requestProblem
	if errors.As(err, &p) {
		fail(w, p.status, p.code, p.message)
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "NOT_FOUND", "Объект не найден")
		return
	}
	var db *pgconn.PgError
	if errors.As(err, &db) {
		switch db.Code {
		case "23503", "23514", "22001", "22P02":
			fail(w, 422, "VALIDATION_ERROR", "Некорректные связанные данные")
			return
		case "23505", "40001", "40P01":
			fail(w, 409, "CONFLICT", "Конфликт состояния; повторите запрос")
			return
		}
	}
	fail(w, 500, "INTERNAL_ERROR", "Не удалось выполнить операцию")
}
func requestUUID(s string) bool {
	var id pgtype.UUID
	return len(s) == 36 && id.Scan(s) == nil && id.Valid
}
func requestString(v any) string { s, _ := v.(string); return s }
func requestAuthorize(r *http.Request, roles string) (*Principal, error) {
	p := current(r)
	if p == nil {
		return nil, requestBad(401, "Не авторизован")
	}
	if !strings.Contains("|"+roles+"|", "|"+p.Role+"|") || p.Role == "" {
		return nil, requestBad(403, "Нет прав")
	}
	return p, nil
}
func requestOwner(p *Principal) (string, string, error) {
	var col, id string
	switch p.Role {
	case "STUDENT":
		col, id = "studentID", p.StudentID
	case "TEACHER":
		col, id = "teacherID", p.TeacherID
	case "DEAN":
		col, id = "createdByEmployerID", p.EmployerID
	}
	if col == "" || !requestUUID(id) {
		return "", "", requestBad(403, "Не определён автор обращения")
	}
	return col, id, nil
}
func requestOwned(p *Principal, v map[string]any) bool {
	col, id, err := requestOwner(p)
	return err == nil && strings.EqualFold(requestString(v[col]), id)
}
func requestPageParams(r *http.Request) (int, int, error) {
	limit, offset := 20, 0
	for _, v := range []struct {
		name     string
		dst      *int
		min, max int
	}{{"limit", &limit, 1, 100}, {"offset", &offset, 0, int(^uint(0) >> 1)}} {
		if values, ok := r.URL.Query()[v.name]; ok {
			if len(values) != 1 {
				return 0, 0, requestBad(400, "Некорректная пагинация")
			}
			n, err := strconv.Atoi(values[0])
			if err != nil || n < v.min || n > v.max {
				return 0, 0, requestBad(400, "Некорректная пагинация")
			}
			*v.dst = n
		}
	}
	return limit, offset, nil
}
func requestOne(ctx context.Context, db requestDB, query string, args ...any) (map[string]any, error) {
	var raw []byte
	err := db.QueryRow(ctx, `SELECT row_to_json(result) FROM (`+query+`) result`, args...).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var v map[string]any
	err = json.Unmarshal(raw, &v)
	return v, err
}
func requestPage(ctx context.Context, db requestDB, table, columns, where, order string, args []any, limit, offset int) (map[string]any, error) {
	// Count and items come from a single statement and therefore one MVCC snapshot.
	query := fmt.Sprintf(`SELECT json_build_object('items',COALESCE((SELECT json_agg(row_to_json(result)) FROM (SELECT %s FROM %s WHERE %s ORDER BY %s LIMIT $%d OFFSET $%d) result),'[]'::json),'total',(SELECT count(*) FROM %s WHERE %s),'limit',$%d::int,'offset',$%d::int)`, columns, table, where, order, len(args)+1, len(args)+2, table, where, len(args)+1, len(args)+2)
	var raw []byte
	params := append(append([]any{}, args...), limit, offset)
	err := db.QueryRow(ctx, query, params...).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var v map[string]any
	err = json.Unmarshal(raw, &v)
	return v, err
}
func requestBody(w http.ResponseWriter, r *http.Request, allowed string) (map[string]json.RawMessage, bool) {
	var body map[string]json.RawMessage
	if !decode(w, r, &body) {
		return nil, false
	}
	if body == nil {
		requestError(w, requestBad(400, "Требуется JSON-объект"))
		return nil, false
	}
	for k := range body {
		if !strings.Contains("|"+allowed+"|", "|"+k+"|") {
			requestError(w, requestBad(400, "Недопустимое поле: "+k))
			return nil, false
		}
	}
	return body, true
}
func requestFields(body map[string]json.RawMessage, draft bool) (map[string]any, error) {
	out := map[string]any{}
	for k, raw := range body {
		if string(raw) == "null" {
			if !draft && k != "buildingID" && k != "subjectStudentID" {
				return nil, requestBad(422, "Поле не может быть null: "+k)
			}
			out[k] = nil
			continue
		}
		var s string
		if json.Unmarshal(raw, &s) != nil {
			return nil, requestBad(422, "Ожидается строка: "+k)
		}
		if strings.HasSuffix(k, "ID") && !requestUUID(s) {
			return nil, requestBad(422, "Некорректный UUID: "+k)
		}
		if k == "appointmentTitle" && utf8.RuneCountInString(s) > 100 {
			return nil, requestBad(422, "Название длиннее 100 символов")
		}
		out[k] = s
	}
	if !draft {
		for _, k := range []string{"appointmentTypeID", "appointmentTitle", "appointmentDescription"} {
			if strings.TrimSpace(requestString(out[k])) == "" {
				return nil, requestBad(422, "Обязательное поле: "+k)
			}
		}
	}
	return out, nil
}
func requestActiveType(ctx context.Context, db requestDB, id any) error {
	if id == nil {
		return nil
	}
	var active bool
	err := db.QueryRow(ctx, `SELECT "isActive" FROM appointment_types WHERE "appointmentTypeID"=$1 FOR SHARE`, id).Scan(&active)
	if errors.Is(err, pgx.ErrNoRows) {
		return requestBad(422, "Тип заявки не найден")
	}
	if err != nil {
		return err
	}
	if !active {
		return requestBad(422, "Тип заявки неактивен")
	}
	return nil
}
func requestHistory(ctx context.Context, db requestDB, id, status, account string, reason any) error {
	_, err := db.Exec(ctx, `INSERT INTO appointment_history ("appointmentHistoryID","appointmentID","appointmentStatus","changedAt",reason,"changedByAccountID") VALUES (gen_random_uuid(),$1,$2,now(),$3,$4)`, id, status, reason, account)
	return err
}
func requestInsert(ctx context.Context, db requestDB, p *Principal, fields map[string]any, fixedIDs ...string) (map[string]any, error) {
	var fixedID any
	if len(fixedIDs) > 0 {
		fixedID = fixedIDs[0]
	}
	col, owner, err := requestOwner(p)
	if err != nil {
		return nil, err
	}
	if err = requestActiveType(ctx, db, fields["appointmentTypeID"]); err != nil {
		return nil, err
	}
	// The contract requires an approved type policy for subjectStudentID, but the
	// supplied schema/docs contain no such policy. Never infer privileges from titles.
	if fields["subjectStudentID"] != nil {
		if p.Role != "DEAN" {
			return nil, requestBad(403, "Указывать студента может только деканат")
		}
		return nil, requestBad(422, "Не настроен перечень согласованных типов для subjectStudentID")
	}
	v, err := requestReturning(ctx, db, `INSERT INTO appointment ("appoinmentID","`+col+`","appointmentTypeID","appointmentTitle","appointmentDescription","buildingID","subjectStudentID","fileUrlAttachment","appointmentStatus",created_at) VALUES (COALESCE($8::uuid,gen_random_uuid()),$1,$2,$3,$4,$5,$6,$7,'creating',now()) RETURNING `+requestColumns, owner, fields["appointmentTypeID"], fields["appointmentTitle"], fields["appointmentDescription"], fields["buildingID"], fields["subjectStudentID"], fields["fileUrlAttachment"], fixedID)
	if err != nil {
		return nil, err
	}
	err = requestHistory(ctx, db, requestString(v["appoinmentID"]), "creating", p.AccountID, nil)
	return v, err
}

// requestReturning wraps DML in a CTE: INSERT/UPDATE cannot be used directly as
// a subquery of SELECT row_to_json.
func requestReturning(ctx context.Context, db requestDB, query string, args ...any) (map[string]any, error) {
	var raw []byte
	err := db.QueryRow(ctx, `WITH result AS (`+query+`) SELECT row_to_json(result) FROM result`, args...).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var v map[string]any
	err = json.Unmarshal(raw, &v)
	return v, err
}
func (s *Server) registerRequests(mux *http.ServeMux) {
	const prefix = "/api/v1/MGIMO"
	routes := map[string]http.HandlerFunc{
		"GET /requests": s.requestList, "POST /requests": s.requestCreate,
		"GET /requests/types":          s.requestTypes,
		"GET /requests/{appoinmentID}": s.requestGet, "GET /requests/{appoinmentID}/status": s.requestGet, "PATCH /requests/{appoinmentID}/status": s.requestMutate,
		"GET /requests/{appoinmentID}/history": s.requestGet, "GET /requests/{appoinmentID}/assignments": s.requestGet,
		"GET /requests/assigned": s.requestList, "GET /requests/unassigned": s.requestList,
		"POST /requests/{appoinmentID}/claim": s.requestMutate, "POST /requests/{appoinmentID}/transfer": s.requestMutate,
		"GET /requests/drafts": s.requestDrafts, "POST /requests/drafts": s.requestDrafts,
		"GET /requests/drafts/{appointmentDraftID}": s.requestDraft, "PATCH /requests/drafts/{appointmentDraftID}": s.requestDraft, "DELETE /requests/drafts/{appointmentDraftID}": s.requestDraft,
		"POST /requests/drafts/{appointmentDraftID}/submit": s.requestDraft,
	}
	// Resolve intersecting wildcard patterns with static segments first.
	patterns := make([]string, 0, len(routes))
	for pattern := range routes {
		patterns = append(patterns, pattern)
	}
	sort.SliceStable(patterns, func(i, j int) bool { return strings.Count(patterns[i], "{") < strings.Count(patterns[j], "{") })
	dispatcher := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodMatch := false
		for _, pattern := range patterns {
			parts := strings.SplitN(pattern, " ", 2)
			full := prefix + parts[1]
			if !contractPathMatch(full, r.URL.Path) {
				continue
			}
			methodMatch = true
			if parts[0] != r.Method {
				continue
			}
			a, b := strings.Split(full, "/"), strings.Split(r.URL.Path, "/")
			for i, segment := range a {
				if strings.HasPrefix(segment, "{") {
					r.SetPathValue(strings.Trim(segment, "{}"), b[i])
				}
			}
			routes[pattern](w, r)
			return
		}
		if methodMatch {
			fail(w, 405, "METHOD_NOT_ALLOWED", "Метод не поддерживается")
		} else {
			fail(w, 404, "NOT_FOUND", "Маршрут не найден")
		}
	})
	mux.Handle(prefix+"/requests", dispatcher)
	mux.Handle(prefix+"/requests/", dispatcher)

	s.registerAttachments(mux)
}
func (s *Server) requestCreate(w http.ResponseWriter, r *http.Request) {
	p, err := requestAuthorize(r, "STUDENT|TEACHER|DEAN")
	if err != nil {
		requestError(w, err)
		return
	}
	body, ok := requestBody(w, r, "appointmentTypeID|appointmentTitle|appointmentDescription|buildingID|subjectStudentID")
	if !ok {
		return
	}
	fields, err := requestFields(body, false)
	if err != nil {
		requestError(w, err)
		return
	}
	tx, err := s.DB.Begin(r.Context())
	if err != nil {
		requestError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	v, err := requestInsert(r.Context(), tx, p, fields)
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		requestError(w, err)
		return
	}
	write(w, 201, v)
}
func (s *Server) requestList(w http.ResponseWriter, r *http.Request) {
	support := strings.HasSuffix(r.URL.Path, "/assigned") || strings.HasSuffix(r.URL.Path, "/unassigned")
	roles := "STUDENT|TEACHER|DEAN"
	if support {
		roles = "SUPPORT"
	}
	p, err := requestAuthorize(r, roles)
	if err != nil {
		requestError(w, err)
		return
	}
	limit, offset, err := requestPageParams(r)
	if err != nil {
		requestError(w, err)
		return
	}
	where := "true"
	args := []any{}
	if support {
		if !requestUUID(p.EmployerID) {
			requestError(w, requestBad(403, "Не определён сотрудник"))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/unassigned") {
			where = `"appointmentStatus"='creating' AND "employerID" IS NULL`
		} else {
			where = `"employerID"=$1`
			args = append(args, p.EmployerID)
		}
	} else {
		col, id, e := requestOwner(p)
		if e != nil {
			requestError(w, e)
			return
		}
		where = `"` + col + `"=$1`
		args = append(args, id)
		if values, ok := r.URL.Query()["appointmentStatus"]; ok {
			status := r.URL.Query().Get("appointmentStatus")
			if len(values) != 1 || !requestValidStatus(status) {
				requestError(w, requestBad(400, "Некорректный статус"))
				return
			}
			args = append(args, status)
			where += fmt.Sprintf(` AND "appointmentStatus"=$%d`, len(args))
		}
	}
	if strings.HasSuffix(r.URL.Path, "/unassigned") {
		where += ` AND EXISTS (SELECT 1 FROM employers e WHERE e."employerID"=$1 AND e."employerStatus"='active') AND NOT EXISTS (SELECT 1 FROM appointment busy WHERE busy."employerID"=$1 AND busy."appointmentStatus" IN ('creating','proccess'))`
		args = append(args, p.EmployerID)
	}
	v, err := requestPage(r.Context(), s.DB, "appointment", requestColumns, where, `created_at DESC, "appoinmentID"`, args, limit, offset)
	if err != nil {
		requestError(w, err)
		return
	}
	write(w, 200, v)
}
func requestValidStatus(s string) bool {
	return s == "creating" || s == "proccess" || s == "ready" || s == "canceled"
}
func (s *Server) requestTypes(w http.ResponseWriter, r *http.Request) {
	if _, err := requestAuthorize(r, "STUDENT|TEACHER|DEAN"); err != nil {
		requestError(w, err)
		return
	}
	limit, offset, err := requestPageParams(r)
	if err != nil {
		requestError(w, err)
		return
	}
	active := true
	if values, ok := r.URL.Query()["activeOnly"]; ok {
		if len(values) != 1 || (values[0] != "true" && values[0] != "false") {
			requestError(w, requestBad(400, "Некорректный activeOnly"))
			return
		}
		active = values[0] == "true"
	}
	v, err := requestPage(r.Context(), s.DB, "appointment_types", requestTypeColumns, `(NOT $1::boolean OR "isActive")`, `"appointmentTypeTitle", "appointmentTypeID"`, []any{active}, limit, offset)
	if err != nil {
		requestError(w, err)
		return
	}
	write(w, 200, v)
}
func requestAccess(ctx context.Context, db requestDB, p *Principal, v map[string]any) error {
	if requestOwned(p, v) {
		return nil
	}
	if p.Role == "SUPPORT" && p.EmployerID != "" && strings.EqualFold(requestString(v["employerID"]), p.EmployerID) {
		return nil
	}
	if p.Role == "DEAN" && requestUUID(p.EmployerID) {
		var allowed bool
		err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM deanery_staff ds JOIN faculties f ON f."deaneryID"=ds."deaneryID" WHERE ds."employerID"=$1 AND (EXISTS (SELECT 1 FROM students st WHERE st."facultyID"=f."facultyID" AND (st."studentID"=$2::uuid OR st."studentID"=$3::uuid)) OR EXISTS (SELECT 1 FROM "facultyTeacher" ft WHERE ft."facultyID"=f."facultyID" AND ft."teacherID"=$4::uuid)))`, p.EmployerID, v["studentID"], v["subjectStudentID"], v["teacherID"]).Scan(&allowed)
		if err != nil {
			return err
		}
		if allowed {
			return nil
		}
	}
	return requestBad(403, "Нет прав на обращение")
}
func (s *Server) requestGet(w http.ResponseWriter, r *http.Request) {
	roles := "STUDENT|TEACHER|DEAN|SUPPORT"
	if strings.HasSuffix(r.URL.Path, "/assignments") {
		roles = "SUPPORT|DEAN"
	}
	p, err := requestAuthorize(r, roles)
	if err != nil {
		requestError(w, err)
		return
	}
	id := r.PathValue("appoinmentID")
	if !requestUUID(id) {
		requestError(w, requestBad(400, "Некорректный UUID"))
		return
	}
	// Hold the row through authorization and reading history: transfer cannot revoke
	// access between the permission check and the response query.
	tx, err := s.DB.Begin(r.Context())
	if err != nil {
		requestError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	v, err := requestOne(r.Context(), tx, `SELECT `+requestColumns+` FROM appointment WHERE "appoinmentID"=$1 FOR SHARE`, id)
	if err == nil {
		err = requestAccess(r.Context(), tx, p, v)
	}
	if err != nil {
		requestError(w, err)
		return
	}
	var result any = v
	switch {
	case strings.HasSuffix(r.URL.Path, "/status"):
		result = map[string]any{"appoinmentID": v["appoinmentID"], "appointmentStatus": v["appointmentStatus"], "updated_at": v["updated_at"]}
	case strings.HasSuffix(r.URL.Path, "/history"), strings.HasSuffix(r.URL.Path, "/assignments"):
		limit, offset, e := requestPageParams(r)
		if e != nil {
			requestError(w, e)
			return
		}
		table, cols, order := "appointment_history", requestHistoryColumns, `"changedAt", "appointmentHistoryID"`
		if strings.HasSuffix(r.URL.Path, "/assignments") {
			table, cols, order = "appointment_assignments", requestAssignmentColumns, `"changedAt", "assignmentID"`
		}
		result, err = requestPage(r.Context(), tx, table, cols, `"appointmentID"=$1`, order, []any{id}, limit, offset)
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		requestError(w, err)
		return
	}
	write(w, 200, result)
}
func (s *Server) requestDrafts(w http.ResponseWriter, r *http.Request) {
	p, err := requestAuthorize(r, "STUDENT|TEACHER|DEAN")
	if err != nil {
		requestError(w, err)
		return
	}
	col, id, err := requestOwner(p)
	if err != nil {
		requestError(w, err)
		return
	}
	if r.Method == http.MethodGet {
		limit, offset, e := requestPageParams(r)
		if e != nil {
			requestError(w, e)
			return
		}
		v, e := requestPage(r.Context(), s.DB, "appointment_drafts", requestDraftColumns, `"`+col+`"=$1 AND "draftStatus"='editing'`, `"updatedAt" DESC, "appointmentDraftID"`, []any{id}, limit, offset)
		if e != nil {
			requestError(w, e)
			return
		}
		write(w, 200, v)
		return
	}
	body, ok := requestBody(w, r, "appointmentTypeID|appointmentTitle|appointmentDescription")
	if !ok {
		return
	}
	fields, err := requestFields(body, true)
	if err != nil {
		requestError(w, err)
		return
	}
	tx, err := s.DB.Begin(r.Context())
	if err != nil {
		requestError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	if err = requestActiveType(r.Context(), tx, fields["appointmentTypeID"]); err != nil {
		requestError(w, err)
		return
	}
	v, err := requestReturning(r.Context(), tx, `INSERT INTO appointment_drafts ("appointmentDraftID","`+col+`","appointmentTypeID","appointmentTitle","appointmentDescription","draftStatus","createdAt","updatedAt") VALUES (gen_random_uuid(),$1,$2,$3,$4,'editing',now(),now()) RETURNING `+requestDraftColumns, id, fields["appointmentTypeID"], fields["appointmentTitle"], fields["appointmentDescription"])
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		requestError(w, err)
		return
	}
	write(w, 201, v)
}
func (s *Server) requestDraft(w http.ResponseWriter, r *http.Request) {
	p, err := requestAuthorize(r, "STUDENT|TEACHER|DEAN")
	if err != nil {
		requestError(w, err)
		return
	}
	id := r.PathValue("appointmentDraftID")
	if !requestUUID(id) {
		requestError(w, requestBad(400, "Некорректный UUID"))
		return
	}
	var fields map[string]any
	var delivery map[string]any
	if strings.HasSuffix(r.URL.Path, "/submit") && r.ContentLength != 0 {
		body, ok := requestBody(w, r, "buildingID")
		if !ok {
			return
		}
		delivery, err = requestFields(body, true)
		if err != nil {
			requestError(w, err)
			return
		}
	}
	if r.Method == http.MethodPatch {
		body, ok := requestBody(w, r, "appointmentTypeID|appointmentTitle|appointmentDescription")
		if !ok {
			return
		}
		if len(body) == 0 {
			requestError(w, requestBad(422, "Не указаны изменения"))
			return
		}
		fields, err = requestFields(body, true)
		if err != nil {
			requestError(w, err)
			return
		}
	}
	tx, err := s.DB.Begin(r.Context())
	if err != nil {
		requestError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	v, err := requestOne(r.Context(), tx, `SELECT `+requestDraftColumns+` FROM appointment_drafts WHERE "appointmentDraftID"=$1 FOR UPDATE`, id)
	if err != nil {
		requestError(w, err)
		return
	}
	if !requestOwned(p, v) {
		requestError(w, requestBad(403, "Нет прав на черновик"))
		return
	}
	status := 200
	if r.Method != http.MethodGet {
		if v["draftStatus"] != "editing" {
			requestError(w, requestBad(409, "Черновик уже отправлен"))
			return
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/submit"):
			v, err = requestSubmit(r.Context(), tx, p, id, v, delivery)
			status = 201
		case r.Method == http.MethodDelete:
			_, err = tx.Exec(r.Context(), `DELETE FROM appointment_drafts WHERE "appointmentDraftID"=$1`, id)
			status = 204
		case r.Method == http.MethodPatch:
			if err = requestActiveType(r.Context(), tx, fields["appointmentTypeID"]); err == nil {
				sets := []string{`"updatedAt"=now()`}
				args := []any{id}
				for _, k := range []string{"appointmentTypeID", "appointmentTitle", "appointmentDescription"} {
					if value, ok := fields[k]; ok {
						args = append(args, value)
						sets = append(sets, fmt.Sprintf(`"%s"=$%d`, k, len(args)))
					}
				}
				v, err = requestReturning(r.Context(), tx, `UPDATE appointment_drafts SET `+strings.Join(sets, ",")+` WHERE "appointmentDraftID"=$1 RETURNING `+requestDraftColumns, args...)
			}
		}
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		requestError(w, err)
		return
	}
	if status == 204 {
		w.WriteHeader(204)
		return
	}
	write(w, status, v)
}
func requestTransition(old, next, reason string) error {
	if next != "proccess" && next != "ready" && next != "canceled" {
		return requestBad(422, "Некорректный новый статус")
	}
	if utf8.RuneCountInString(reason) > 564 || (next == "canceled" && strings.TrimSpace(reason) == "") {
		return requestBad(422, "Требуется причина отмены длиной до 564 символов")
	}
	if !(old == "creating" && (next == "proccess" || next == "canceled") || old == "proccess" && (next == "ready" || next == "canceled")) {
		return requestBad(409, "Недопустимый переход статуса")
	}
	return nil
}

// All assignment mutations acquire employer rows in UUID order before locking
// the appointment. This is the common lock order for claim/transfer/status and
// serializes the busy check even when two different appointments are claimed.
func requestLockSupport(ctx context.Context, tx pgx.Tx, ids ...string) error {
	for _, id := range ids {
		if !requestUUID(id) {
			return requestBad(403, "Не определён сотрудник")
		}
	}
	rows, err := tx.Query(ctx, `SELECT "employerID"::text,"employerStatus"::text FROM employers WHERE "employerID"::text=ANY($1::text[]) ORDER BY "employerID" FOR UPDATE`, ids)
	if err != nil {
		return err
	}
	found := map[string]bool{}
	for rows.Next() {
		var id, status string
		if err = rows.Scan(&id, &status); err != nil {
			rows.Close()
			return err
		}
		found[strings.ToLower(id)] = status == "active"
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if !found[strings.ToLower(id)] {
			return requestBad(409, "Сотрудник недоступен")
		}
	}
	// Lock linked account rows too: a transfer must not target an inactive account
	// or an employee whose account is not SUPPORT.
	for _, id := range ids {
		var account string
		err = tx.QueryRow(ctx, `SELECT "accountID"::text FROM accounts WHERE "employerID"=$1 AND "accountRole"='SUPPORT' AND "isActive" FOR SHARE`, id).Scan(&account)
		if errors.Is(err, pgx.ErrNoRows) {
			return requestBad(409, "Нет активной учётной записи поддержки")
		}
		if err != nil {
			return err
		}
	}
	return nil
}
func requestFree(ctx context.Context, db requestDB, id string) error {
	var busy bool
	err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM appointment WHERE "employerID"=$1 AND "appointmentStatus" IN ('creating','proccess'))`, id).Scan(&busy)
	if err != nil {
		return err
	}
	if busy {
		return requestBad(409, "Сотрудник занят другой заявкой")
	}
	return nil
}
func (s *Server) requestMutate(w http.ResponseWriter, r *http.Request) {
	p, err := requestAuthorize(r, "SUPPORT")
	if err != nil {
		requestError(w, err)
		return
	}
	id := r.PathValue("appoinmentID")
	if !requestUUID(id) {
		requestError(w, requestBad(400, "Некорректный UUID"))
		return
	}
	action := "status"
	if strings.HasSuffix(r.URL.Path, "/claim") {
		action = "claim"
	}
	if strings.HasSuffix(r.URL.Path, "/transfer") {
		action = "transfer"
	}
	target := p.EmployerID
	next := "proccess"
	var reason any
	if action != "claim" {
		allowed := "appointmentStatus|reason"
		if action == "transfer" {
			allowed = "employerID"
		}
		body, ok := requestBody(w, r, allowed)
		if !ok {
			return
		}
		if action == "transfer" {
			if json.Unmarshal(body["employerID"], &target) != nil || !requestUUID(target) {
				requestError(w, requestBad(422, "Требуется employerID"))
				return
			}
			if strings.EqualFold(target, p.EmployerID) {
				requestError(w, requestBad(409, "Заявка уже назначена этому сотруднику"))
				return
			}
		}
		if action == "status" {
			if json.Unmarshal(body["appointmentStatus"], &next) != nil {
				requestError(w, requestBad(422, "Требуется appointmentStatus"))
				return
			}
			text := ""
			if raw, ok := body["reason"]; ok && string(raw) != "null" {
				if json.Unmarshal(raw, &text) != nil {
					requestError(w, requestBad(422, "Некорректная причина"))
					return
				}
				reason = text
			}
			if e := requestTransition("creating", next, text); e != nil {
				var problem *requestProblem
				if errors.As(e, &problem) && problem.status == 422 {
					requestError(w, e)
					return
				}
			}
		}
	}
	target = strings.ToLower(target)
	tx, err := s.DB.Begin(r.Context())
	if err != nil {
		requestError(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	v, err := requestApply(r.Context(), tx, p, id, action, target, next, reason)
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		requestError(w, err)
		return
	}
	write(w, 200, v)
}

func requestApply(ctx context.Context, tx pgx.Tx, p *Principal, id, action, target, next string, reason any) (map[string]any, error) {
	ids := []string{strings.ToLower(p.EmployerID)}
	if target != strings.ToLower(p.EmployerID) {
		ids = append(ids, target)
	}
	if err := requestLockSupport(ctx, tx, ids...); err != nil {
		return nil, err
	}
	v, err := requestOne(ctx, tx, `SELECT `+requestColumns+` FROM appointment WHERE "appoinmentID"=$1 FOR UPDATE`, id)
	if err != nil {
		return nil, err
	}
	old := requestString(v["appointmentStatus"])
	oldEmployer := v["employerID"]
	if action == "claim" {
		if old != "creating" || oldEmployer != nil {
			return nil, requestBad(409, "Заявка уже принята или закрыта")
		}
	} else {
		if !strings.EqualFold(requestString(oldEmployer), p.EmployerID) {
			return nil, requestBad(403, "Заявка не назначена текущему сотруднику")
		}
		if action == "transfer" {
			if old != "creating" && old != "proccess" {
				return nil, requestBad(409, "Закрытую заявку нельзя передать")
			}
			next = old
		} else {
			if err = requestTransition(old, next, requestString(reason)); err != nil {
				return nil, err
			}
		}
	}
	if action == "claim" || action == "transfer" {
		if err = requestFree(ctx, tx, target); err != nil {
			return nil, err
		}
	}
	v, err = requestReturning(ctx, tx, `UPDATE appointment SET "employerID"=$2,"appointmentStatus"=$3,updated_at=now() WHERE "appoinmentID"=$1 RETURNING `+requestColumns, id, target, next)
	if err == nil && action != "transfer" {
		err = requestHistory(ctx, tx, id, next, p.AccountID, reason)
	}
	if err == nil && action != "status" {
		_, err = tx.Exec(ctx, `INSERT INTO appointment_assignments ("assignmentID","appointmentID","oldEmployerID","newEmployerID","changedByAccountID","changedAt") VALUES (gen_random_uuid(),$1,$2,$3,$4,now())`, id, oldEmployer, target, p.AccountID)
	}
	return v, err
}

// Caller holds the draft row lock until commit; submitted is a durable guard.
func requestSubmit(ctx context.Context, tx pgx.Tx, p *Principal, id string, draft map[string]any, delivery ...map[string]any) (map[string]any, error) {
	if !requestOwned(p, draft) {
		return nil, requestBad(403, "Нет прав на черновик")
	}
	if draft["draftStatus"] != "editing" {
		return nil, requestBad(409, "Черновик уже отправлен")
	}
	body := map[string]json.RawMessage{}
	for _, k := range []string{"appointmentTypeID", "appointmentTitle", "appointmentDescription"} {
		body[k], _ = json.Marshal(draft[k])
	}
	fields, err := requestFields(body, false)
	if err != nil {
		return nil, err
	}
	fields["fileUrlAttachment"] = draft["fileUrlAttachment"]
	if len(delivery) > 0 {
		fields["buildingID"] = delivery[0]["buildingID"]
	}
	result, err := requestInsert(ctx, tx, p, fields, id)
	if err != nil {
		return nil, err
	}
	tag, err := tx.Exec(ctx, `UPDATE appointment_drafts SET "draftStatus"='submitted',"updatedAt"=now() WHERE "appointmentDraftID"=$1 AND "draftStatus"='editing'`, id)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() != 1 {
		return nil, requestBad(409, "Черновик уже отправлен")
	}
	return result, nil
}
