package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"mgimo/backend/internal/password"
)

func demoID(n int) string { return fmt.Sprintf("00000000-0000-4000-8000-%012d", n) }
func seed(ctx context.Context, conn *pgxpool.Conn, secret string) error {
	tx, e := conn.Begin(ctx)
	if e != nil {
		return databaseError("начало заполнения", e)
	}
	defer rollback(tx)
	h, e := password.Hash(secret)
	if e != nil {
		return e
	}
	random := make([]byte, 32)
	if _, e = rand.Read(random); e != nil {
		return e
	}
	unused, e := password.Hash(hex.EncodeToString(random))
	if e != nil {
		return e
	}
	passport := []any{}
	for i := 0; i < 7; i++ {
		v, e := password.Hash(fmt.Sprintf("СИНТЕТИЧЕСКОЕ ПОЛЕ %d НЕ ДОКУМЕНТ", i))
		if e != nil {
			return e
		}
		passport = append(passport, v)
	}
	insert := func(table, columns string, values ...any) error {
		names := strings.Split(columns, ",")
		quoted := make([]string, len(names))
		marks := make([]string, len(names))
		for i, v := range names {
			quoted[i] = `"` + v + `"`
			marks[i] = fmt.Sprintf("$%d", i+1)
		}
		args := append([]any{pgx.QueryExecModeSimpleProtocol}, values...)
		_, err := tx.Exec(ctx, `INSERT INTO "`+table+`" (`+strings.Join(quoted, ",")+`) VALUES (`+strings.Join(marks, ",")+`) ON CONFLICT DO NOTHING`, args...)
		if err != nil {
			return databaseError("демонстрационная таблица "+table, err)
		}
		return nil
	}
	must := func(e error) {
		if e != nil {
			panic(seedFailure{e})
		}
	}
	// Recover only our own sentinel to keep the transaction's rollback guaranteed.
	return seedRows(ctx, tx, insert, must, h, unused, passport)
}

type seedFailure struct{ error }

func seedRows(ctx context.Context, tx pgx.Tx, insert func(string, string, ...any) error, must func(error), h, unused string, passport []any) (err error) {
	defer func() {
		if v := recover(); v != nil {
			if e, ok := v.(seedFailure); ok {
				err = e.error
			} else {
				panic(v)
			}
		}
	}()
	now := time.Now().UTC()
	birthday := time.Date(2005, 2, 12, 0, 0, 0, 0, time.UTC)
	enrolled := time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)
	must(insert("groups", "groupID,groupNumber", 1, 101))
	must(insert("groups", "groupID,groupNumber", 2, 102))
	for i, role := range []string{"SUPPORT", "DEAN", "ADMIN"} {
		values := []any{demoID(30 + i), 300 + i, "Вымышленный сотрудник " + role, fmt.Sprintf("+7000000030%d", i), strings.ToLower(role) + "@example.test"}
		values = append(values, passport...)
		values = append(values, "active", enrolled)
		must(insert("employers", "employerID,employerNumber,employerName,employerPhoneNumber,employerEmail,passportSeria,passportNumber,passportWhoIssued,passportDataIssue,passportCodeDivison,passportCityBorn,passportRegistration,employerStatus,hireAt", values...))
		must(insert("accounts", "accountID,email,passwordHash,accountRole,employerID", demoID(130+i), strings.ToLower(role)+"@example.test", h, role, demoID(30+i)))
	}
	must(insert("deaneries", "deaneryID,name,email,deanID", 1, "Демонстрационный деканат", "deanery@example.test", demoID(31)))
	must(insert("faculties", "facultyID,deaneryID,title,description", demoID(1), 1, "Учебный факультет", "Только вымышленные данные"))
	must(insert("direction", "directionID,facultyID,directionName", demoID(2), demoID(1), "Информационные системы"))
	must(insert("deanery_staff", "deaneryID,employerID", 1, demoID(31)))
	for i, name := range []string{"Алексей Демонстрационный", "Мария Примерова"} {
		values := []any{demoID(10 + i), 2026001 + i, name, fmt.Sprintf("student%d@example.test", i+1), unused, fmt.Sprintf("+7000000010%d", i), birthday}
		values = append(values, passport...)
		values = append(values, "active", enrolled, "Н/Д", i+1, demoID(1), demoID(2))
		must(insert("students", "studentID,studentNumber,studentName,studentEmail,studentPassword,studentPhoneNumber,studentBirthday,passportSeria,passportNumber,passportWhoIssued,passportDataIssue,passportCodeDivison,passportCityBorn,passportRegistration,studentStatus,studentEnrollmentDate,studentGender,groupID,facultyID,directionID", values...))
		must(insert("accounts", "accountID,email,passwordHash,accountRole,studentID", demoID(110+i), fmt.Sprintf("student%d@example.test", i+1), h, "STUDENT", demoID(10+i)))
	}
	must(insert("teachers", "teacherID,teacherNumber,teacherName,teacherPosition,teacherDegree,teacherEmail,teacherPassword,teacherPhoneNumber,teacherBirthday,teacherStatus", demoID(20), 2001, "Виктор Учебный", "Преподаватель", "Кандидат наук", "teacher@example.test", unused, "+70000000200", birthday, "active"))
	must(insert("accounts", "accountID,email,passwordHash,accountRole,teacherID", demoID(120), "teacher@example.test", h, "TEACHER", demoID(20)))
	must(insert("facultyTeacher", "facultyID,teacherID", demoID(1), demoID(20)))
	must(insert("courses", "courseID,facultyID,title,description,hours,semester", demoID(40), demoID(1), "Базы данных", "Проектирование и SQL", 72, 1))
	must(insert("courseTeacher", "courseID,teacherID", demoID(40), demoID(20)))
	for i := 0; i < 2; i++ {
		must(insert("courseGroup", "courseGroupID,courseID,groupID,start_date,end_date", demoID(50+i), demoID(40), i+1, enrolled, enrolled.AddDate(4, 0, 0)))
	}
	must(insert("buildings", "buildingID,building,buildingAddress", demoID(60), "Учебный корпус", "Москва, проспект Вернадского, 76"))
	must(insert("classroom", "classroomID,buildingID,classroomNumber", 1, demoID(60), 101))
	start := time.Date(now.Year(), now.Month(), now.Day(), 10, 0, 0, 0, time.FixedZone("MSK", 3*60*60))
	for i := 0; i < 2; i++ {
		must(insert("lessons", "lessonID,groupID,classroomID,teacherID,topic,startAt,endAt,lessonStatus,courseID", i+1, i+1, 1, demoID(20), "Связи и ограничения целостности", start.Add(time.Duration(i)*2*time.Hour), start.Add(time.Duration(i)*2*time.Hour+90*time.Minute), "active", demoID(40)))
	}
	must(insert("deadlines", "deadlineID,courseID,deadlineTitle,deadlineDescription,deadlineStartAt,deadlineEndAt", 1, demoID(40), "Лабораторная работа", "Подготовить схему данных", now, now.Add(7*24*time.Hour)))
	for i, score := range []int{85, 59} {
		must(insert("courseScore", "courseGraduate,courseID,studentID,teacherID,score,credit,created_at,updated_at", demoID(70+i), demoID(40), demoID(10+i), demoID(20), score, score >= 60, now, now))
	}
	must(insert("appointment_types", "appointmentTypeID,appointmentTypeTitle,appointmentTypeDescription", demoID(80), "Справка об обучении", "Запросить учебную справку"))
	must(insert("appointment", "appoinmentID,appointmentTypeID,studentID,appointmentTitle,appointmentDescription,appointmentStatus,created_at", demoID(81), demoID(80), demoID(10), "Справка об обучении", "Вымышленная демонстрационная заявка", "creating", now))
	must(insert("appointment_history", "appointmentHistoryID,appointmentID,appointmentStatus,changedAt,changedByAccountID", demoID(82), demoID(81), "creating", now, demoID(110)))
	must(insert("news", "newsID,deaneryID,employerID,newsTitle,newsDescription,createdAt", 1, 1, demoID(31), "Демонстрационный деканат открыт", "В системе используются только вымышленные данные.", now))
	must(insert("notifications", "notificationID,studentID,notificationTitle,notificationMessage,notificationCategory,newsID", demoID(90), demoID(10), "Добро пожаловать", "Доступен демонстрационный учебный кабинет", "news", 1))
	must(insert("exams", "examID,courseID,groupID,teacherID,classroomID,examTitle,examStartAt,examEndAt", demoID(91), demoID(40), 1, demoID(20), 1, "Экзамен по базам данных", now.Add(14*24*time.Hour), now.Add(14*24*time.Hour+90*time.Minute)))
	must(insert("sports", "sportID,sportTitle,sportDescription,buildingID,sportCapacity,sportStartAt,sportEndAt", demoID(92), "Волейбол", "Демонстрационная секция", demoID(60), 1, now.Add(48*time.Hour), now.Add(49*time.Hour)))
	if e := tx.Commit(ctx); e != nil {
		return databaseError("фиксация демонстрационных данных", e)
	}
	return nil
}
