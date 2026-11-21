-- Источник: docs/database.dbml. Регистр и исходные опечатки сохранены.
BEGIN;
SET LOCAL search_path = public, pg_catalog;

CREATE TYPE public."status_students" AS ENUM ('active', 'expelled', 'academic_leave');

CREATE TYPE public."status_teacher" AS ENUM ('active', 'fired', 'vacation');

CREATE TYPE public."lesson_status" AS ENUM ('active', 'canceled', 'rescheduled');

CREATE TYPE public."appointment_status" AS ENUM ('creating', 'ready', 'proccess', 'canceled');

CREATE TYPE public."employers_status" AS ENUM ('active', 'vacation', 'sick', 'fired');

CREATE TYPE public."directors_status" AS ENUM ('active', 'vacation', 'sick', 'fired');

CREATE TYPE public."exam_status" AS ENUM ('scheduled', 'completed', 'canceled');

CREATE TYPE public."sport_registration_status" AS ENUM ('enrolled', 'canceled');

CREATE TYPE public."appointment_draft_status" AS ENUM ('editing', 'submitted');

CREATE TYPE public."account_role" AS ENUM ('STUDENT', 'TEACHER', 'SUPPORT', 'DEAN', 'ADMIN');

CREATE TYPE public."notification_category" AS ENUM ('news', 'announcement', 'schedule_change', 'deadline', 'request', 'exam', 'sport');

CREATE TABLE public."students" (
    "studentID" uuid PRIMARY KEY,
    "studentNumber" int NOT NULL UNIQUE,
    "studentName" varchar(256) NOT NULL,
    "studentEmail" varchar(258) NOT NULL UNIQUE,
    "studentPassword" varchar(256) NOT NULL,
    "studentPhoneNumber" varchar(29) NOT NULL UNIQUE,
    "studentBirthday" timestamp NOT NULL,
    "passportSeria" varchar(256) NOT NULL,
    "passportNumber" varchar(256) NOT NULL,
    "passportWhoIssued" varchar(256) NOT NULL,
    "passportDataIssue" varchar(256) NOT NULL,
    "passportCodeDivison" varchar(256) NOT NULL,
    "passportCityBorn" varchar(256) NOT NULL,
    "passportRegistration" varchar(256) NOT NULL,
    "studentStatus" public."status_students" NOT NULL,
    "studentEnrollmentDate" timestamp NOT NULL,
    "studentGender" char(3) NOT NULL,
    "groupID" integer NOT NULL,
    "facultyID" uuid,
    "directionID" uuid
);

CREATE TABLE public."notifications" (
    "notificationID" uuid PRIMARY KEY,
    "notificationMessage" varchar(128) NOT NULL,
    "studentID" uuid,
    "employerID" uuid,
    "teacherID" uuid,
    "notificationTitle" varchar(128) NOT NULL,
    "notificationCategory" public."notification_category" NOT NULL,
    "notificationCreatedAt" timestamptz NOT NULL DEFAULT now(),
    "notificationReadAt" timestamptz,
    "lessonID" integer,
    "appoinmentID" uuid,
    "courseID" uuid,
    "examID" uuid,
    "sportID" uuid,
    "newsID" integer,
    CONSTRAINT "notifications_one_recipient" CHECK (num_nonnulls("studentID", "employerID", "teacherID") = 1),
    CONSTRAINT "notifications_one_target" CHECK (num_nonnulls("lessonID", "appoinmentID", "courseID", "examID", "sportID", "newsID") <= 1)
);

CREATE TABLE public."groups" (
    "groupID" integer PRIMARY KEY,
    "groupNumber" integer NOT NULL
);

CREATE TABLE public."teachers" (
    "teacherID" uuid PRIMARY KEY,
    "teacherNumber" int NOT NULL UNIQUE,
    "teacherName" varchar(256) NOT NULL,
    "teacherPosition" varchar(256) NOT NULL,
    "teacherDegree" varchar(256) NOT NULL,
    "teacherEmail" varchar(258) NOT NULL UNIQUE,
    "teacherPassword" varchar(256) NOT NULL,
    "teacherPhoneNumber" varchar(29) NOT NULL UNIQUE,
    "teacherBirthday" timestamp NOT NULL,
    "teacherStatus" public."status_teacher" NOT NULL
);

CREATE TABLE public."courses" (
    "courseID" uuid PRIMARY KEY,
    "facultyID" uuid NOT NULL,
    "title" varchar(128) NOT NULL UNIQUE,
    "description" varchar(564),
    "hours" int NOT NULL,
    "semester" int NOT NULL
);

CREATE TABLE public."deadlines" (
    "deadlineID" int PRIMARY KEY,
    "courseID" uuid,
    "deadlineTitle" varchar(128) NOT NULL,
    "deadlineDescription" text NOT NULL,
    "deadlineStartAt" timestamptz,
    "deadlineEndAt" timestamptz
);

CREATE TABLE public."classroom" (
    "classroomID" integer PRIMARY KEY,
    "buildingID" uuid NOT NULL,
    "classroomNumber" integer NOT NULL
);

CREATE TABLE public."buildings" (
    "buildingID" uuid PRIMARY KEY,
    "building" varchar(256) NOT NULL,
    "buildingAddress" varchar(256)
);

CREATE TABLE public."courseGroup" (
    "courseGroupID" uuid PRIMARY KEY,
    "courseID" uuid NOT NULL,
    "groupID" integer NOT NULL,
    "start_date" timestamptz NOT NULL,
    "end_date" timestamptz NOT NULL
);

CREATE TABLE public."courseTeacher" (
    "courseID" uuid NOT NULL,
    "teacherID" uuid NOT NULL,
    PRIMARY KEY ("courseID", "teacherID")
);

CREATE TABLE public."faculties" (
    "facultyID" uuid PRIMARY KEY,
    "deaneryID" int NOT NULL UNIQUE,
    "title" varchar(128) NOT NULL UNIQUE,
    "description" varchar(564)
);

CREATE TABLE public."direction" (
    "directionID" uuid PRIMARY KEY,
    "facultyID" uuid NOT NULL,
    "directionName" varchar(256) NOT NULL,
    "directionDescription" varchar(564)
);

CREATE TABLE public."facultyTeacher" (
    "facultyID" uuid NOT NULL,
    "teacherID" uuid NOT NULL,
    PRIMARY KEY ("facultyID", "teacherID")
);

CREATE TABLE public."lessons" (
    "lessonID" integer PRIMARY KEY,
    "groupID" integer NOT NULL,
    "classroomID" integer NOT NULL,
    "teacherID" uuid NOT NULL,
    "topic" varchar(256) NOT NULL,
    "startAt" timestamptz NOT NULL,
    "endAt" timestamptz NOT NULL,
    "remoteLessonLink" varchar(1024) UNIQUE,
    "lessonStatus" public."lesson_status" NOT NULL,
    "courseID" uuid NOT NULL
);

CREATE TABLE public."lesson_changes" (
    "lessonChangeID" int PRIMARY KEY,
    "lessonID" integer,
    "changedAt" timestamptz NOT NULL,
    "oldStartAt" timestamptz,
    "oldEndAt" timestamptz,
    "newStartAt" timestamptz,
    "newEndAt" timestamptz,
    "oldRoomID" integer,
    "newRoomID" integer,
    "changedByTeacherID" uuid,
    "oldTeacherID" uuid,
    "newTeacherID" uuid,
    "reason" varchar(564) NOT NULL,
    "changedByEmployerID" uuid,
    "oldLessonStatus" public."lesson_status",
    "newLessonStatus" public."lesson_status",
    "oldRemoteLessonLink" varchar(1024),
    "newRemoteLessonLink" varchar(1024)
);

CREATE TABLE public."courseScore" (
    "courseGraduate" uuid PRIMARY KEY,
    "courseID" uuid NOT NULL,
    "studentID" uuid NOT NULL,
    "teacherID" uuid NOT NULL,
    "score" int CHECK (score >=0 AND score <=100),
    "credit" boolean,
    "created_at" timestamptz NOT NULL,
    "updated_at" timestamptz NOT NULL,
    UNIQUE ("courseID", "studentID"),
    CONSTRAINT "course_score_credit_consistency" CHECK (((score IS NULL AND credit IS NULL) OR (score IS NOT NULL AND credit IS NOT NULL AND credit = (score >= 60))))
);

CREATE TABLE public."appointment" (
    "appoinmentID" uuid PRIMARY KEY,
    "appointmentTypeID" uuid,
    "studentID" uuid,
    "employerID" uuid,
    "buildingID" uuid,
    "appointmentTitle" varchar(100) NOT NULL,
    "appointmentDescription" text NOT NULL,
    "fileUrlAttachment" varchar(256) UNIQUE,
    "appointmentStatus" public."appointment_status" NOT NULL,
    "created_at" timestamptz NOT NULL,
    "updated_at" timestamptz,
    "teacherID" uuid,
    "createdByEmployerID" uuid,
    "subjectStudentID" uuid,
    CONSTRAINT "appointment_one_author" CHECK (num_nonnulls("studentID", "teacherID", "createdByEmployerID") = 1)
);

CREATE TABLE public."appointment_history" (
    "appointmentHistoryID" uuid PRIMARY KEY,
    "appointmentID" uuid NOT NULL,
    "appointmentStatus" public."appointment_status" NOT NULL,
    "changedAt" timestamptz NOT NULL DEFAULT now(),
    "reason" varchar(564),
    "changedByAccountID" uuid
);

CREATE TABLE public."employers" (
    "employerID" uuid PRIMARY KEY,
    "employerNumber" int NOT NULL UNIQUE,
    "employerName" varchar(256) NOT NULL,
    "employerPhoneNumber" varchar(29) NOT NULL UNIQUE,
    "employerEmail" varchar(256) NOT NULL UNIQUE,
    "passportSeria" varchar(256) NOT NULL,
    "passportNumber" varchar(256) NOT NULL,
    "passportWhoIssued" varchar(256) NOT NULL,
    "passportDataIssue" varchar(256) NOT NULL,
    "passportCodeDivison" varchar(256) NOT NULL,
    "passportCityBorn" varchar(256) NOT NULL,
    "passportRegistration" varchar(256) NOT NULL,
    "departamentID" uuid,
    "employerStatus" public."employers_status" NOT NULL,
    "hireAt" timestamptz NOT NULL,
    "fireAt" timestamptz
);

CREATE TABLE public."departament" (
    "departamentID" uuid PRIMARY KEY,
    "directorID" uuid NOT NULL,
    "departamentName" varchar(256) NOT NULL
);

CREATE TABLE public."director" (
    "directorID" uuid PRIMARY KEY,
    "directorName" varchar(256) NOT NULL,
    "directorBirthday" timestamp,
    "directorPhoneNumber" varchar(29) NOT NULL UNIQUE,
    "directorEmail" varchar(256) NOT NULL UNIQUE,
    "passportSeria" varchar(256) NOT NULL,
    "passportNumber" varchar(256) NOT NULL,
    "passportWhoIssued" varchar(256) NOT NULL,
    "passportDataIssue" varchar(256) NOT NULL,
    "passportCodeDivison" varchar(256) NOT NULL,
    "passportCityBorn" varchar(256) NOT NULL,
    "passportRegistration" varchar(256) NOT NULL,
    "directorStatus" public."directors_status" NOT NULL,
    "hireAt" timestamptz NOT NULL,
    "fireAt" timestamptz
);

CREATE TABLE public."news" (
    "newsID" integer PRIMARY KEY,
    "deaneryID" int,
    "employerID" uuid,
    "newsTitle" varchar(128) NOT NULL,
    "newsDescription" TEXT NOT NULL,
    "createdAt" timestamptz,
    "updatedAt" timestamptz
);

CREATE TABLE public."deaneries" (
    "deaneryID" int PRIMARY KEY,
    "name" varchar(150) NOT NULL,
    "email" varchar(256),
    "phone" varchar(29),
    "officeNumber" varchar(29),
    "workingHours" varchar(255),
    "deanID" uuid
);

CREATE TABLE public."exams" (
    "examID" uuid PRIMARY KEY,
    "courseID" uuid NOT NULL,
    "groupID" integer NOT NULL,
    "teacherID" uuid NOT NULL,
    "classroomID" integer,
    "examTitle" varchar(128) NOT NULL,
    "examStartAt" timestamptz NOT NULL,
    "examEndAt" timestamptz NOT NULL,
    "examStatus" public."exam_status" NOT NULL DEFAULT 'scheduled',
    CONSTRAINT "exam_valid_time" CHECK ("examEndAt" > "examStartAt")
);

CREATE TABLE public."exam_results" (
    "examResultID" uuid PRIMARY KEY,
    "examID" uuid NOT NULL,
    "studentID" uuid NOT NULL,
    "score" integer CHECK (score >= 0 AND score <= 100),
    "createdAt" timestamptz NOT NULL,
    "updatedAt" timestamptz,
    UNIQUE ("examID", "studentID")
);

CREATE TABLE public."sports" (
    "sportID" uuid PRIMARY KEY,
    "sportTitle" varchar(128) NOT NULL,
    "sportDescription" text,
    "buildingID" uuid,
    "sportCapacity" integer NOT NULL CHECK ("sportCapacity" > 0),
    "sportStartAt" timestamptz NOT NULL,
    "sportEndAt" timestamptz NOT NULL,
    CONSTRAINT "sport_valid_time" CHECK ("sportEndAt" > "sportStartAt")
);

CREATE TABLE public."sport_registrations" (
    "sportRegistrationID" uuid PRIMARY KEY,
    "sportID" uuid NOT NULL,
    "studentID" uuid NOT NULL,
    "registrationStatus" public."sport_registration_status" NOT NULL DEFAULT 'enrolled',
    "createdAt" timestamptz NOT NULL,
    "updatedAt" timestamptz,
    UNIQUE ("sportID", "studentID")
);

CREATE TABLE public."appointment_types" (
    "appointmentTypeID" uuid PRIMARY KEY,
    "appointmentTypeTitle" varchar(100) NOT NULL UNIQUE,
    "appointmentTypeDescription" text,
    "isActive" boolean NOT NULL DEFAULT true
);

CREATE TABLE public."appointment_drafts" (
    "appointmentDraftID" uuid PRIMARY KEY,
    "studentID" uuid,
    "appointmentTypeID" uuid,
    "appointmentTitle" varchar(100),
    "appointmentDescription" text,
    "fileUrlAttachment" varchar(256),
    "draftStatus" public."appointment_draft_status" NOT NULL DEFAULT 'editing',
    "createdAt" timestamptz NOT NULL,
    "updatedAt" timestamptz NOT NULL,
    "teacherID" uuid,
    "createdByEmployerID" uuid,
    CONSTRAINT "appointment_draft_one_author" CHECK (num_nonnulls("studentID", "teacherID", "createdByEmployerID") = 1)
);

CREATE TABLE public."accounts" (
    "accountID" uuid PRIMARY KEY,
    "email" varchar(258) NOT NULL UNIQUE,
    "passwordHash" varchar(256) NOT NULL,
    "accountRole" public."account_role" NOT NULL,
    "studentID" uuid UNIQUE,
    "teacherID" uuid UNIQUE,
    "employerID" uuid UNIQUE,
    "isActive" boolean NOT NULL DEFAULT true,
    "createdAt" timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT "account_role_matches_person" CHECK ((("accountRole" = 'STUDENT' AND "studentID" IS NOT NULL AND "teacherID" IS NULL AND "employerID" IS NULL) OR ("accountRole" = 'TEACHER' AND "teacherID" IS NOT NULL AND "studentID" IS NULL AND "employerID" IS NULL) OR ("accountRole" IN ('SUPPORT','DEAN','ADMIN') AND "employerID" IS NOT NULL AND "studentID" IS NULL AND "teacherID" IS NULL)))
);

CREATE TABLE public."user_sessions" (
    "sessionID" uuid PRIMARY KEY,
    "accountID" uuid NOT NULL,
    "refreshTokenHash" varchar(256) NOT NULL UNIQUE,
    "createdAt" timestamptz NOT NULL DEFAULT now(),
    "expiresAt" timestamptz NOT NULL,
    "revokedAt" timestamptz,
    CONSTRAINT "session_valid_expiration" CHECK ("expiresAt" > "createdAt")
);

CREATE TABLE public."deanery_staff" (
    "deaneryID" int NOT NULL,
    "employerID" uuid NOT NULL,
    PRIMARY KEY ("deaneryID", "employerID")
);

CREATE TABLE public."appointment_assignments" (
    "assignmentID" uuid PRIMARY KEY,
    "appointmentID" uuid NOT NULL,
    "oldEmployerID" uuid,
    "newEmployerID" uuid NOT NULL,
    "changedByAccountID" uuid NOT NULL,
    "changedAt" timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE public."exams" ADD CONSTRAINT "exams_courseID_fkey" FOREIGN KEY ("courseID")
    REFERENCES public."courses" ("courseID") ON UPDATE CASCADE ON DELETE RESTRICT;
CREATE INDEX "exams_courseID_idx" ON public."exams" ("courseID");

ALTER TABLE public."exams" ADD CONSTRAINT "exams_groupID_fkey" FOREIGN KEY ("groupID")
    REFERENCES public."groups" ("groupID") ON UPDATE CASCADE ON DELETE RESTRICT;
CREATE INDEX "exams_groupID_idx" ON public."exams" ("groupID");

ALTER TABLE public."exams" ADD CONSTRAINT "exams_teacherID_fkey" FOREIGN KEY ("teacherID")
    REFERENCES public."teachers" ("teacherID") ON UPDATE CASCADE ON DELETE RESTRICT;
CREATE INDEX "exams_teacherID_idx" ON public."exams" ("teacherID");

ALTER TABLE public."exams" ADD CONSTRAINT "exams_classroomID_fkey" FOREIGN KEY ("classroomID")
    REFERENCES public."classroom" ("classroomID") ON UPDATE CASCADE ON DELETE SET NULL;
CREATE INDEX "exams_classroomID_idx" ON public."exams" ("classroomID");

ALTER TABLE public."exam_results" ADD CONSTRAINT "exam_results_examID_fkey" FOREIGN KEY ("examID")
    REFERENCES public."exams" ("examID") ON UPDATE CASCADE ON DELETE RESTRICT;

ALTER TABLE public."exam_results" ADD CONSTRAINT "exam_results_studentID_fkey" FOREIGN KEY ("studentID")
    REFERENCES public."students" ("studentID") ON UPDATE CASCADE ON DELETE RESTRICT;
CREATE INDEX "exam_results_studentID_idx" ON public."exam_results" ("studentID");

ALTER TABLE public."sports" ADD CONSTRAINT "sports_buildingID_fkey" FOREIGN KEY ("buildingID")
    REFERENCES public."buildings" ("buildingID") ON UPDATE CASCADE ON DELETE SET NULL;
CREATE INDEX "sports_buildingID_idx" ON public."sports" ("buildingID");

ALTER TABLE public."sport_registrations" ADD CONSTRAINT "sport_registrations_sportID_fkey" FOREIGN KEY ("sportID")
    REFERENCES public."sports" ("sportID") ON UPDATE CASCADE ON DELETE CASCADE;

ALTER TABLE public."sport_registrations" ADD CONSTRAINT "sport_registrations_studentID_fkey" FOREIGN KEY ("studentID")
    REFERENCES public."students" ("studentID") ON UPDATE CASCADE ON DELETE CASCADE;
CREATE INDEX "sport_registrations_studentID_idx" ON public."sport_registrations" ("studentID");

ALTER TABLE public."appointment" ADD CONSTRAINT "appointment_appointmentTypeID_fkey" FOREIGN KEY ("appointmentTypeID")
    REFERENCES public."appointment_types" ("appointmentTypeID") ON UPDATE CASCADE ON DELETE SET NULL;
CREATE INDEX "appointment_appointmentTypeID_idx" ON public."appointment" ("appointmentTypeID");

ALTER TABLE public."appointment_drafts" ADD CONSTRAINT "appointment_drafts_studentID_fkey" FOREIGN KEY ("studentID")
    REFERENCES public."students" ("studentID") ON UPDATE CASCADE ON DELETE RESTRICT;
CREATE INDEX "appointment_drafts_studentID_idx" ON public."appointment_drafts" ("studentID");

ALTER TABLE public."appointment_drafts" ADD CONSTRAINT "appointment_drafts_appointmentTypeID_fkey" FOREIGN KEY ("appointmentTypeID")
    REFERENCES public."appointment_types" ("appointmentTypeID") ON UPDATE CASCADE ON DELETE SET NULL;
CREATE INDEX "appointment_drafts_appointmentTypeID_idx" ON public."appointment_drafts" ("appointmentTypeID");

ALTER TABLE public."courseGroup" ADD CONSTRAINT "courseGroup_courseID_fkey" FOREIGN KEY ("courseID")
    REFERENCES public."courses" ("courseID") ON UPDATE CASCADE ON DELETE CASCADE;
CREATE INDEX "courseGroup_courseID_idx" ON public."courseGroup" ("courseID");

ALTER TABLE public."courseTeacher" ADD CONSTRAINT "courseTeacher_teacherID_fkey" FOREIGN KEY ("teacherID")
    REFERENCES public."teachers" ("teacherID") ON UPDATE CASCADE ON DELETE CASCADE;
CREATE INDEX "courseTeacher_teacherID_idx" ON public."courseTeacher" ("teacherID");

ALTER TABLE public."courseTeacher" ADD CONSTRAINT "courseTeacher_courseID_fkey" FOREIGN KEY ("courseID")
    REFERENCES public."courses" ("courseID") ON UPDATE CASCADE ON DELETE CASCADE;

ALTER TABLE public."facultyTeacher" ADD CONSTRAINT "facultyTeacher_facultyID_fkey" FOREIGN KEY ("facultyID")
    REFERENCES public."faculties" ("facultyID") ON UPDATE CASCADE ON DELETE CASCADE;

ALTER TABLE public."facultyTeacher" ADD CONSTRAINT "facultyTeacher_teacherID_fkey" FOREIGN KEY ("teacherID")
    REFERENCES public."teachers" ("teacherID") ON UPDATE CASCADE ON DELETE CASCADE;
CREATE INDEX "facultyTeacher_teacherID_idx" ON public."facultyTeacher" ("teacherID");

ALTER TABLE public."direction" ADD CONSTRAINT "direction_facultyID_fkey" FOREIGN KEY ("facultyID")
    REFERENCES public."faculties" ("facultyID") ON UPDATE CASCADE ON DELETE RESTRICT;
CREATE INDEX "direction_facultyID_idx" ON public."direction" ("facultyID");

ALTER TABLE public."students" ADD CONSTRAINT "students_groupID_fkey" FOREIGN KEY ("groupID")
    REFERENCES public."groups" ("groupID") ON UPDATE CASCADE ON DELETE RESTRICT;
CREATE INDEX "students_groupID_idx" ON public."students" ("groupID");

ALTER TABLE public."courseGroup" ADD CONSTRAINT "courseGroup_groupID_fkey" FOREIGN KEY ("groupID")
    REFERENCES public."groups" ("groupID") ON UPDATE CASCADE ON DELETE CASCADE;
CREATE INDEX "courseGroup_groupID_idx" ON public."courseGroup" ("groupID");

ALTER TABLE public."lessons" ADD CONSTRAINT "lessons_classroomID_fkey" FOREIGN KEY ("classroomID")
    REFERENCES public."classroom" ("classroomID") ON UPDATE CASCADE ON DELETE RESTRICT;
CREATE INDEX "lessons_classroomID_idx" ON public."lessons" ("classroomID");

ALTER TABLE public."lessons" ADD CONSTRAINT "lessons_groupID_fkey" FOREIGN KEY ("groupID")
    REFERENCES public."groups" ("groupID") ON UPDATE CASCADE ON DELETE RESTRICT;
CREATE INDEX "lessons_groupID_idx" ON public."lessons" ("groupID");

ALTER TABLE public."lessons" ADD CONSTRAINT "lessons_teacherID_fkey" FOREIGN KEY ("teacherID")
    REFERENCES public."teachers" ("teacherID") ON UPDATE CASCADE ON DELETE RESTRICT;
CREATE INDEX "lessons_teacherID_idx" ON public."lessons" ("teacherID");

ALTER TABLE public."lesson_changes" ADD CONSTRAINT "lesson_changes_lessonID_fkey" FOREIGN KEY ("lessonID")
    REFERENCES public."lessons" ("lessonID") ON UPDATE CASCADE ON DELETE SET NULL;
CREATE INDEX "lesson_changes_lessonID_idx" ON public."lesson_changes" ("lessonID");

ALTER TABLE public."lesson_changes" ADD CONSTRAINT "lesson_changes_oldRoomID_fkey" FOREIGN KEY ("oldRoomID")
    REFERENCES public."classroom" ("classroomID") ON UPDATE CASCADE ON DELETE SET NULL;
CREATE INDEX "lesson_changes_oldRoomID_idx" ON public."lesson_changes" ("oldRoomID");

ALTER TABLE public."lesson_changes" ADD CONSTRAINT "lesson_changes_newRoomID_fkey" FOREIGN KEY ("newRoomID")
    REFERENCES public."classroom" ("classroomID") ON UPDATE CASCADE ON DELETE SET NULL;
CREATE INDEX "lesson_changes_newRoomID_idx" ON public."lesson_changes" ("newRoomID");

ALTER TABLE public."lesson_changes" ADD CONSTRAINT "lesson_changes_oldTeacherID_fkey" FOREIGN KEY ("oldTeacherID")
    REFERENCES public."teachers" ("teacherID") ON UPDATE CASCADE ON DELETE SET NULL;
CREATE INDEX "lesson_changes_oldTeacherID_idx" ON public."lesson_changes" ("oldTeacherID");

ALTER TABLE public."lesson_changes" ADD CONSTRAINT "lesson_changes_newTeacherID_fkey" FOREIGN KEY ("newTeacherID")
    REFERENCES public."teachers" ("teacherID") ON UPDATE CASCADE ON DELETE SET NULL;
CREATE INDEX "lesson_changes_newTeacherID_idx" ON public."lesson_changes" ("newTeacherID");

ALTER TABLE public."lesson_changes" ADD CONSTRAINT "lesson_changes_changedByTeacherID_fkey" FOREIGN KEY ("changedByTeacherID")
    REFERENCES public."teachers" ("teacherID") ON UPDATE CASCADE ON DELETE SET NULL;
CREATE INDEX "lesson_changes_changedByTeacherID_idx" ON public."lesson_changes" ("changedByTeacherID");

ALTER TABLE public."courseScore" ADD CONSTRAINT "courseScore_studentID_fkey" FOREIGN KEY ("studentID")
    REFERENCES public."students" ("studentID") ON UPDATE CASCADE ON DELETE RESTRICT;
CREATE INDEX "courseScore_studentID_idx" ON public."courseScore" ("studentID");

ALTER TABLE public."courseScore" ADD CONSTRAINT "courseScore_courseID_fkey" FOREIGN KEY ("courseID")
    REFERENCES public."courses" ("courseID") ON UPDATE CASCADE ON DELETE RESTRICT;

ALTER TABLE public."courseScore" ADD CONSTRAINT "courseScore_teacherID_fkey" FOREIGN KEY ("teacherID")
    REFERENCES public."teachers" ("teacherID") ON UPDATE CASCADE ON DELETE RESTRICT;
CREATE INDEX "courseScore_teacherID_idx" ON public."courseScore" ("teacherID");

ALTER TABLE public."appointment" ADD CONSTRAINT "appointment_studentID_fkey" FOREIGN KEY ("studentID")
    REFERENCES public."students" ("studentID") ON UPDATE CASCADE ON DELETE RESTRICT;
CREATE INDEX "appointment_studentID_idx" ON public."appointment" ("studentID");

ALTER TABLE public."appointment_history" ADD CONSTRAINT "appointment_history_appointmentID_fkey" FOREIGN KEY ("appointmentID")
    REFERENCES public."appointment" ("appoinmentID") ON UPDATE CASCADE ON DELETE RESTRICT;
CREATE INDEX "appointment_history_appointmentID_idx" ON public."appointment_history" ("appointmentID");

ALTER TABLE public."departament" ADD CONSTRAINT "departament_directorID_fkey" FOREIGN KEY ("directorID")
    REFERENCES public."director" ("directorID") ON UPDATE CASCADE ON DELETE RESTRICT;
CREATE INDEX "departament_directorID_idx" ON public."departament" ("directorID");

ALTER TABLE public."employers" ADD CONSTRAINT "employers_departamentID_fkey" FOREIGN KEY ("departamentID")
    REFERENCES public."departament" ("departamentID") ON UPDATE CASCADE ON DELETE SET NULL;
CREATE INDEX "employers_departamentID_idx" ON public."employers" ("departamentID");

ALTER TABLE public."appointment" ADD CONSTRAINT "appointment_employerID_fkey" FOREIGN KEY ("employerID")
    REFERENCES public."employers" ("employerID") ON UPDATE CASCADE ON DELETE SET NULL;
CREATE INDEX "appointment_employerID_idx" ON public."appointment" ("employerID");

ALTER TABLE public."deaneries" ADD CONSTRAINT "deaneries_deanID_fkey" FOREIGN KEY ("deanID")
    REFERENCES public."employers" ("employerID") ON UPDATE CASCADE ON DELETE SET NULL;
CREATE INDEX "deaneries_deanID_idx" ON public."deaneries" ("deanID");

ALTER TABLE public."news" ADD CONSTRAINT "news_employerID_fkey" FOREIGN KEY ("employerID")
    REFERENCES public."employers" ("employerID") ON UPDATE CASCADE ON DELETE SET NULL;
CREATE INDEX "news_employerID_idx" ON public."news" ("employerID");

ALTER TABLE public."news" ADD CONSTRAINT "news_deaneryID_fkey" FOREIGN KEY ("deaneryID")
    REFERENCES public."deaneries" ("deaneryID") ON UPDATE CASCADE ON DELETE SET NULL;
CREATE INDEX "news_deaneryID_idx" ON public."news" ("deaneryID");

ALTER TABLE public."students" ADD CONSTRAINT "students_facultyID_fkey" FOREIGN KEY ("facultyID")
    REFERENCES public."faculties" ("facultyID") ON UPDATE CASCADE ON DELETE SET NULL;
CREATE INDEX "students_facultyID_idx" ON public."students" ("facultyID");

ALTER TABLE public."faculties" ADD CONSTRAINT "faculties_deaneryID_fkey" FOREIGN KEY ("deaneryID")
    REFERENCES public."deaneries" ("deaneryID") ON UPDATE CASCADE ON DELETE RESTRICT;

ALTER TABLE public."deadlines" ADD CONSTRAINT "deadlines_courseID_fkey" FOREIGN KEY ("courseID")
    REFERENCES public."courses" ("courseID") ON UPDATE CASCADE ON DELETE SET NULL;
CREATE INDEX "deadlines_courseID_idx" ON public."deadlines" ("courseID");

ALTER TABLE public."classroom" ADD CONSTRAINT "classroom_buildingID_fkey" FOREIGN KEY ("buildingID")
    REFERENCES public."buildings" ("buildingID") ON UPDATE CASCADE ON DELETE RESTRICT;
CREATE INDEX "classroom_buildingID_idx" ON public."classroom" ("buildingID");

ALTER TABLE public."appointment" ADD CONSTRAINT "appointment_buildingID_fkey" FOREIGN KEY ("buildingID")
    REFERENCES public."buildings" ("buildingID") ON UPDATE CASCADE ON DELETE SET NULL;
CREATE INDEX "appointment_buildingID_idx" ON public."appointment" ("buildingID");

ALTER TABLE public."notifications" ADD CONSTRAINT "notifications_studentID_fkey" FOREIGN KEY ("studentID")
    REFERENCES public."students" ("studentID") ON UPDATE CASCADE ON DELETE CASCADE;
CREATE INDEX "notifications_studentID_idx" ON public."notifications" ("studentID");

ALTER TABLE public."notifications" ADD CONSTRAINT "notifications_employerID_fkey" FOREIGN KEY ("employerID")
    REFERENCES public."employers" ("employerID") ON UPDATE CASCADE ON DELETE CASCADE;
CREATE INDEX "notifications_employerID_idx" ON public."notifications" ("employerID");

ALTER TABLE public."courses" ADD CONSTRAINT "courses_facultyID_fkey" FOREIGN KEY ("facultyID")
    REFERENCES public."faculties" ("facultyID") ON UPDATE CASCADE ON DELETE RESTRICT;
CREATE INDEX "courses_facultyID_idx" ON public."courses" ("facultyID");

ALTER TABLE public."students" ADD CONSTRAINT "students_directionID_fkey" FOREIGN KEY ("directionID")
    REFERENCES public."direction" ("directionID") ON UPDATE CASCADE ON DELETE SET NULL;
CREATE INDEX "students_directionID_idx" ON public."students" ("directionID");

ALTER TABLE public."lessons" ADD CONSTRAINT "lessons_courseID_fkey" FOREIGN KEY ("courseID")
    REFERENCES public."courses" ("courseID") ON UPDATE CASCADE ON DELETE RESTRICT;
CREATE INDEX "lessons_courseID_idx" ON public."lessons" ("courseID");

ALTER TABLE public."appointment" ADD CONSTRAINT "appointment_teacherID_fkey" FOREIGN KEY ("teacherID")
    REFERENCES public."teachers" ("teacherID") ON UPDATE CASCADE ON DELETE RESTRICT;
CREATE INDEX "appointment_teacherID_idx" ON public."appointment" ("teacherID");

ALTER TABLE public."appointment" ADD CONSTRAINT "appointment_createdByEmployerID_fkey" FOREIGN KEY ("createdByEmployerID")
    REFERENCES public."employers" ("employerID") ON UPDATE CASCADE ON DELETE RESTRICT;
CREATE INDEX "appointment_createdByEmployerID_idx" ON public."appointment" ("createdByEmployerID");

ALTER TABLE public."appointment" ADD CONSTRAINT "appointment_subjectStudentID_fkey" FOREIGN KEY ("subjectStudentID")
    REFERENCES public."students" ("studentID") ON UPDATE CASCADE ON DELETE SET NULL;
CREATE INDEX "appointment_subjectStudentID_idx" ON public."appointment" ("subjectStudentID");

ALTER TABLE public."appointment_drafts" ADD CONSTRAINT "appointment_drafts_teacherID_fkey" FOREIGN KEY ("teacherID")
    REFERENCES public."teachers" ("teacherID") ON UPDATE CASCADE ON DELETE RESTRICT;
CREATE INDEX "appointment_drafts_teacherID_idx" ON public."appointment_drafts" ("teacherID");

ALTER TABLE public."appointment_drafts" ADD CONSTRAINT "appointment_drafts_createdByEmployerID_fkey" FOREIGN KEY ("createdByEmployerID")
    REFERENCES public."employers" ("employerID") ON UPDATE CASCADE ON DELETE RESTRICT;
CREATE INDEX "appointment_drafts_createdByEmployerID_idx" ON public."appointment_drafts" ("createdByEmployerID");

ALTER TABLE public."notifications" ADD CONSTRAINT "notifications_teacherID_fkey" FOREIGN KEY ("teacherID")
    REFERENCES public."teachers" ("teacherID") ON UPDATE CASCADE ON DELETE CASCADE;
CREATE INDEX "notifications_teacherID_idx" ON public."notifications" ("teacherID");

ALTER TABLE public."notifications" ADD CONSTRAINT "notifications_lessonID_fkey" FOREIGN KEY ("lessonID")
    REFERENCES public."lessons" ("lessonID") ON UPDATE CASCADE ON DELETE CASCADE;
CREATE INDEX "notifications_lessonID_idx" ON public."notifications" ("lessonID");

ALTER TABLE public."notifications" ADD CONSTRAINT "notifications_appoinmentID_fkey" FOREIGN KEY ("appoinmentID")
    REFERENCES public."appointment" ("appoinmentID") ON UPDATE CASCADE ON DELETE CASCADE;
CREATE INDEX "notifications_appoinmentID_idx" ON public."notifications" ("appoinmentID");

ALTER TABLE public."notifications" ADD CONSTRAINT "notifications_courseID_fkey" FOREIGN KEY ("courseID")
    REFERENCES public."courses" ("courseID") ON UPDATE CASCADE ON DELETE CASCADE;
CREATE INDEX "notifications_courseID_idx" ON public."notifications" ("courseID");

ALTER TABLE public."notifications" ADD CONSTRAINT "notifications_examID_fkey" FOREIGN KEY ("examID")
    REFERENCES public."exams" ("examID") ON UPDATE CASCADE ON DELETE CASCADE;
CREATE INDEX "notifications_examID_idx" ON public."notifications" ("examID");

ALTER TABLE public."notifications" ADD CONSTRAINT "notifications_sportID_fkey" FOREIGN KEY ("sportID")
    REFERENCES public."sports" ("sportID") ON UPDATE CASCADE ON DELETE CASCADE;
CREATE INDEX "notifications_sportID_idx" ON public."notifications" ("sportID");

ALTER TABLE public."notifications" ADD CONSTRAINT "notifications_newsID_fkey" FOREIGN KEY ("newsID")
    REFERENCES public."news" ("newsID") ON UPDATE CASCADE ON DELETE CASCADE;
CREATE INDEX "notifications_newsID_idx" ON public."notifications" ("newsID");

ALTER TABLE public."lesson_changes" ADD CONSTRAINT "lesson_changes_changedByEmployerID_fkey" FOREIGN KEY ("changedByEmployerID")
    REFERENCES public."employers" ("employerID") ON UPDATE CASCADE ON DELETE SET NULL;
CREATE INDEX "lesson_changes_changedByEmployerID_idx" ON public."lesson_changes" ("changedByEmployerID");

ALTER TABLE public."appointment_history" ADD CONSTRAINT "appointment_history_changedByAccountID_fkey" FOREIGN KEY ("changedByAccountID")
    REFERENCES public."accounts" ("accountID") ON UPDATE CASCADE ON DELETE SET NULL;
CREATE INDEX "appointment_history_changedByAccountID_idx" ON public."appointment_history" ("changedByAccountID");

ALTER TABLE public."accounts" ADD CONSTRAINT "accounts_studentID_fkey" FOREIGN KEY ("studentID")
    REFERENCES public."students" ("studentID") ON UPDATE CASCADE ON DELETE CASCADE;

ALTER TABLE public."accounts" ADD CONSTRAINT "accounts_teacherID_fkey" FOREIGN KEY ("teacherID")
    REFERENCES public."teachers" ("teacherID") ON UPDATE CASCADE ON DELETE CASCADE;

ALTER TABLE public."accounts" ADD CONSTRAINT "accounts_employerID_fkey" FOREIGN KEY ("employerID")
    REFERENCES public."employers" ("employerID") ON UPDATE CASCADE ON DELETE CASCADE;

ALTER TABLE public."user_sessions" ADD CONSTRAINT "user_sessions_accountID_fkey" FOREIGN KEY ("accountID")
    REFERENCES public."accounts" ("accountID") ON UPDATE CASCADE ON DELETE CASCADE;
CREATE INDEX "user_sessions_accountID_idx" ON public."user_sessions" ("accountID");

ALTER TABLE public."deanery_staff" ADD CONSTRAINT "deanery_staff_deaneryID_fkey" FOREIGN KEY ("deaneryID")
    REFERENCES public."deaneries" ("deaneryID") ON UPDATE CASCADE ON DELETE CASCADE;

ALTER TABLE public."deanery_staff" ADD CONSTRAINT "deanery_staff_employerID_fkey" FOREIGN KEY ("employerID")
    REFERENCES public."employers" ("employerID") ON UPDATE CASCADE ON DELETE CASCADE;
CREATE INDEX "deanery_staff_employerID_idx" ON public."deanery_staff" ("employerID");

ALTER TABLE public."appointment_assignments" ADD CONSTRAINT "appointment_assignments_appointmentID_fkey" FOREIGN KEY ("appointmentID")
    REFERENCES public."appointment" ("appoinmentID") ON UPDATE CASCADE ON DELETE RESTRICT;
CREATE INDEX "appointment_assignments_appointmentID_idx" ON public."appointment_assignments" ("appointmentID");

ALTER TABLE public."appointment_assignments" ADD CONSTRAINT "appointment_assignments_oldEmployerID_fkey" FOREIGN KEY ("oldEmployerID")
    REFERENCES public."employers" ("employerID") ON UPDATE CASCADE ON DELETE SET NULL;
CREATE INDEX "appointment_assignments_oldEmployerID_idx" ON public."appointment_assignments" ("oldEmployerID");

ALTER TABLE public."appointment_assignments" ADD CONSTRAINT "appointment_assignments_newEmployerID_fkey" FOREIGN KEY ("newEmployerID")
    REFERENCES public."employers" ("employerID") ON UPDATE CASCADE ON DELETE RESTRICT;
CREATE INDEX "appointment_assignments_newEmployerID_idx" ON public."appointment_assignments" ("newEmployerID");

ALTER TABLE public."appointment_assignments" ADD CONSTRAINT "appointment_assignments_changedByAccountID_fkey" FOREIGN KEY ("changedByAccountID")
    REFERENCES public."accounts" ("accountID") ON UPDATE CASCADE ON DELETE RESTRICT;
CREATE INDEX "appointment_assignments_changedByAccountID_idx" ON public."appointment_assignments" ("changedByAccountID");

-- Закрытый доступ: политики RLS намеренно отсутствуют.
ALTER TABLE public."students" ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public."students" FROM PUBLIC;
ALTER TABLE public."notifications" ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public."notifications" FROM PUBLIC;
ALTER TABLE public."groups" ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public."groups" FROM PUBLIC;
ALTER TABLE public."teachers" ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public."teachers" FROM PUBLIC;
ALTER TABLE public."courses" ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public."courses" FROM PUBLIC;
ALTER TABLE public."deadlines" ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public."deadlines" FROM PUBLIC;
ALTER TABLE public."classroom" ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public."classroom" FROM PUBLIC;
ALTER TABLE public."buildings" ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public."buildings" FROM PUBLIC;
ALTER TABLE public."courseGroup" ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public."courseGroup" FROM PUBLIC;
ALTER TABLE public."courseTeacher" ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public."courseTeacher" FROM PUBLIC;
ALTER TABLE public."faculties" ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public."faculties" FROM PUBLIC;
ALTER TABLE public."direction" ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public."direction" FROM PUBLIC;
ALTER TABLE public."facultyTeacher" ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public."facultyTeacher" FROM PUBLIC;
ALTER TABLE public."lessons" ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public."lessons" FROM PUBLIC;
ALTER TABLE public."lesson_changes" ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public."lesson_changes" FROM PUBLIC;
ALTER TABLE public."courseScore" ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public."courseScore" FROM PUBLIC;
ALTER TABLE public."appointment" ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public."appointment" FROM PUBLIC;
ALTER TABLE public."appointment_history" ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public."appointment_history" FROM PUBLIC;
ALTER TABLE public."employers" ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public."employers" FROM PUBLIC;
ALTER TABLE public."departament" ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public."departament" FROM PUBLIC;
ALTER TABLE public."director" ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public."director" FROM PUBLIC;
ALTER TABLE public."news" ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public."news" FROM PUBLIC;
ALTER TABLE public."deaneries" ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public."deaneries" FROM PUBLIC;
ALTER TABLE public."exams" ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public."exams" FROM PUBLIC;
ALTER TABLE public."exam_results" ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public."exam_results" FROM PUBLIC;
ALTER TABLE public."sports" ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public."sports" FROM PUBLIC;
ALTER TABLE public."sport_registrations" ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public."sport_registrations" FROM PUBLIC;
ALTER TABLE public."appointment_types" ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public."appointment_types" FROM PUBLIC;
ALTER TABLE public."appointment_drafts" ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public."appointment_drafts" FROM PUBLIC;
ALTER TABLE public."accounts" ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public."accounts" FROM PUBLIC;
ALTER TABLE public."user_sessions" ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public."user_sessions" FROM PUBLIC;
ALTER TABLE public."deanery_staff" ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public."deanery_staff" FROM PUBLIC;
ALTER TABLE public."appointment_assignments" ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public."appointment_assignments" FROM PUBLIC;

DO $security$
DECLARE
    role_name text;
    table_name text;
BEGIN
    FOREACH role_name IN ARRAY ARRAY['anon', 'authenticated'] LOOP
        IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = role_name) THEN
            FOREACH table_name IN ARRAY ARRAY['students', 'notifications', 'groups', 'teachers', 'courses', 'deadlines', 'classroom', 'buildings', 'courseGroup', 'courseTeacher', 'faculties', 'direction', 'facultyTeacher', 'lessons', 'lesson_changes', 'courseScore', 'appointment', 'appointment_history', 'employers', 'departament', 'director', 'news', 'deaneries', 'exams', 'exam_results', 'sports', 'sport_registrations', 'appointment_types', 'appointment_drafts', 'accounts', 'user_sessions', 'deanery_staff', 'appointment_assignments'] LOOP
                EXECUTE format('REVOKE ALL PRIVILEGES ON TABLE public.%I FROM %I', table_name, role_name);
            END LOOP;
        END IF;
    END LOOP;
END;
$security$;

COMMIT;
