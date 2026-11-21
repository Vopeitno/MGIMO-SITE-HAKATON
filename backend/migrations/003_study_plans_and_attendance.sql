-- Curriculum, personal stream choices and attendance-based physical education.
-- Existing columns are preserved. New groups/lessons use dedicated sequences.
CREATE SEQUENCE public.study_group_id_seq AS integer START 1000000;
CREATE SEQUENCE public.study_lesson_id_seq AS integer START 1000000;
SELECT setval('public.study_group_id_seq',GREATEST(1000000,COALESCE((SELECT max("groupID")+1 FROM public.groups),1000000)),false);
SELECT setval('public.study_lesson_id_seq',GREATEST(1000000,COALESCE((SELECT max("lessonID")+1 FROM public.lessons),1000000)),false);
CREATE TABLE public.study_plan_periods (
 "periodID" uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 "facultyID" uuid NOT NULL REFERENCES public.faculties("facultyID") ON UPDATE CASCADE ON DELETE CASCADE,
 "academicYear" integer NOT NULL CHECK("academicYear" BETWEEN 1990 AND 2200),
 "semester" integer NOT NULL CHECK("semester" BETWEEN 1 AND 20),
 "opensAt" timestamptz NOT NULL,
 "closesAt" timestamptz NOT NULL,
 "isPublished" boolean NOT NULL DEFAULT false,
 "createdByEmployerID" uuid REFERENCES public.employers("employerID") ON UPDATE CASCADE ON DELETE SET NULL,
 "createdAt" timestamptz NOT NULL DEFAULT now(),
 CHECK("closesAt">"opensAt"),
 UNIQUE("facultyID","academicYear","semester")
);
CREATE TABLE public.study_plan_requirements (
 "requirementID" uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 "periodID" uuid NOT NULL REFERENCES public.study_plan_periods("periodID") ON UPDATE CASCADE ON DELETE CASCADE,
 "courseID" uuid NOT NULL REFERENCES public.courses("courseID") ON UPDATE CASCADE ON DELETE CASCADE,
 "directionID" uuid REFERENCES public.direction("directionID") ON UPDATE CASCADE ON DELETE CASCADE,
 "kind" text NOT NULL CHECK("kind" IN ('common','profile','physical_education')),
 CHECK (("kind"='profile' AND "directionID" IS NOT NULL) OR ("kind" IN ('common','physical_education') AND "directionID" IS NULL))
);
CREATE UNIQUE INDEX study_requirement_identity_idx ON public.study_plan_requirements("periodID","courseID",COALESCE("directionID",'00000000-0000-0000-0000-000000000000'::uuid));
CREATE UNIQUE INDEX study_one_pe_course_idx ON public.study_plan_requirements("periodID") WHERE "kind"='physical_education';
CREATE UNIQUE INDEX course_group_stream_identity_idx ON public."courseGroup"("courseGroupID","courseID");
CREATE TABLE public.course_streams (
 "streamID" uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 "periodID" uuid NOT NULL REFERENCES public.study_plan_periods("periodID") ON UPDATE CASCADE ON DELETE CASCADE,
 "courseID" uuid NOT NULL REFERENCES public.courses("courseID") ON UPDATE CASCADE ON DELETE CASCADE,
 "courseGroupID" uuid NOT NULL UNIQUE,
 "teacherID" uuid NOT NULL REFERENCES public.teachers("teacherID") ON UPDATE CASCADE ON DELETE RESTRICT,
 "capacity" integer NOT NULL CHECK("capacity" BETWEEN 1 AND 10000),
 "isOpen" boolean NOT NULL DEFAULT true,
 "remoteLessonLink" varchar(2048),
 "createdAt" timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY("courseGroupID","courseID") REFERENCES public."courseGroup"("courseGroupID","courseID") ON UPDATE CASCADE ON DELETE CASCADE,
 UNIQUE("streamID","periodID","courseID")
);
CREATE INDEX course_streams_period_idx ON public.course_streams("periodID");
CREATE TABLE public.student_stream_selections (
 "studentID" uuid NOT NULL REFERENCES public.students("studentID") ON UPDATE CASCADE ON DELETE CASCADE,
 "periodID" uuid NOT NULL REFERENCES public.study_plan_periods("periodID") ON UPDATE CASCADE ON DELETE CASCADE,
 "courseID" uuid NOT NULL,
 "streamID" uuid NOT NULL,
 "selectedAt" timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY("studentID","periodID","courseID"),
 FOREIGN KEY("streamID","periodID","courseID") REFERENCES public.course_streams("streamID","periodID","courseID") ON UPDATE CASCADE ON DELETE CASCADE
);
CREATE INDEX student_stream_selections_stream_idx ON public.student_stream_selections("streamID");
CREATE TABLE public.sport_instructors (
 "sportID" uuid NOT NULL REFERENCES public.sports("sportID") ON UPDATE CASCADE ON DELETE CASCADE,
 "teacherID" uuid NOT NULL REFERENCES public.teachers("teacherID") ON UPDATE CASCADE ON DELETE CASCADE,
 "assignedByEmployerID" uuid REFERENCES public.employers("employerID") ON UPDATE CASCADE ON DELETE SET NULL,
 "assignedAt" timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY("sportID","teacherID")
);
CREATE INDEX sport_instructors_teacher_idx ON public.sport_instructors("teacherID");
CREATE TABLE public.sport_attendance (
 "attendanceID" uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 "studentID" uuid NOT NULL REFERENCES public.students("studentID") ON UPDATE CASCADE ON DELETE CASCADE,
 "sportID" uuid NOT NULL REFERENCES public.sports("sportID") ON UPDATE CASCADE ON DELETE CASCADE,
 "teacherID" uuid REFERENCES public.teachers("teacherID") ON UPDATE CASCADE ON DELETE SET NULL,
 "present" boolean NOT NULL DEFAULT true,
 "attendedAt" timestamptz NOT NULL,
 "weekStart" date GENERATED ALWAYS AS (date_trunc('week',"attendedAt" AT TIME ZONE 'Europe/Moscow')::date) STORED,
 "weeklySlot" smallint NOT NULL CHECK("weeklySlot" BETWEEN 1 AND 2),
 "markedAt" timestamptz NOT NULL DEFAULT now(),
 "updatedAt" timestamptz NOT NULL DEFAULT now(),
 UNIQUE("studentID","sportID")
);
CREATE UNIQUE INDEX sport_two_visits_per_week_idx ON public.sport_attendance("studentID","weekStart","weeklySlot") WHERE "present";
CREATE INDEX sport_attendance_student_date_idx ON public.sport_attendance("studentID","attendedAt") WHERE "present";
CREATE TABLE public.sport_attendance_history (
 "historyID" uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 "attendanceID" uuid NOT NULL REFERENCES public.sport_attendance("attendanceID") ON UPDATE CASCADE ON DELETE CASCADE,
 "teacherID" uuid REFERENCES public.teachers("teacherID") ON UPDATE CASCADE ON DELETE SET NULL,
 "present" boolean NOT NULL,
 "changedAt" timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sport_attendance_history_attendance_idx ON public.sport_attendance_history("attendanceID","changedAt");
CREATE FUNCTION public.study_term_start(y integer,s integer) RETURNS timestamptz LANGUAGE sql STABLE AS $term$
 SELECT make_timestamptz(CASE WHEN s%2=1 THEN y ELSE y+1 END,CASE WHEN s%2=1 THEN 9 ELSE 2 END,1,0,0,0,'Europe/Moscow')
$term$;
CREATE FUNCTION public.study_term_end(y integer,s integer) RETURNS timestamptz LANGUAGE sql STABLE AS $term$
 SELECT make_timestamptz(y+1,CASE WHEN s%2=1 THEN 2 ELSE 9 END,1,0,0,0,'Europe/Moscow')
$term$;
CREATE VIEW public.student_course_access AS
 SELECT st."studentID",cg."courseID" FROM public.students st JOIN public."courseGroup" cg ON cg."groupID"=st."groupID"
 UNION
 SELECT sel."studentID",sel."courseID" FROM public.student_stream_selections sel
 UNION
 SELECT st."studentID",r."courseID" FROM public.study_plan_periods p JOIN public.study_plan_requirements r ON r."periodID"=p."periodID" AND r.kind='physical_education' JOIN public.students st ON st."facultyID"=p."facultyID" WHERE p."isPublished" AND p."academicYear"=extract(year FROM st."studentEnrollmentDate")::integer+(p.semester-1)/2;
CREATE VIEW public.student_lesson_access AS
 SELECT st."studentID",l."lessonID" FROM public.students st JOIN public.lessons l ON l."groupID"=st."groupID"
 WHERE NOT EXISTS(SELECT 1 FROM public.student_stream_selections sel JOIN public.study_plan_periods p ON p."periodID"=sel."periodID" WHERE sel."studentID"=st."studentID" AND sel."courseID"=l."courseID" AND l."startAt">=public.study_term_start(p."academicYear",p.semester) AND l."startAt"<public.study_term_end(p."academicYear",p.semester))
 UNION
 SELECT sel."studentID",l."lessonID" FROM public.student_stream_selections sel JOIN public.course_streams cs ON cs."streamID"=sel."streamID" JOIN public."courseGroup" cg ON cg."courseGroupID"=cs."courseGroupID" JOIN public.lessons l ON l."groupID"=cg."groupID" AND l."courseID"=cs."courseID" JOIN public.study_plan_periods p ON p."periodID"=sel."periodID" WHERE l."startAt">=public.study_term_start(p."academicYear",p.semester) AND l."startAt"<public.study_term_end(p."academicYear",p.semester) AND l."startAt">=cg.start_date AND l."startAt"<=cg.end_date;
CREATE VIEW public.student_exam_access AS
 SELECT st."studentID",e."examID" FROM public.students st JOIN public.exams e ON e."groupID"=st."groupID"
 WHERE NOT EXISTS(SELECT 1 FROM public.student_stream_selections sel JOIN public.study_plan_periods p ON p."periodID"=sel."periodID" WHERE sel."studentID"=st."studentID" AND sel."courseID"=e."courseID" AND e."examStartAt">=public.study_term_start(p."academicYear",p.semester) AND e."examStartAt"<public.study_term_end(p."academicYear",p.semester))
 UNION
 SELECT sel."studentID",e."examID" FROM public.student_stream_selections sel JOIN public.course_streams cs ON cs."streamID"=sel."streamID" JOIN public."courseGroup" cg ON cg."courseGroupID"=cs."courseGroupID" JOIN public.exams e ON e."groupID"=cg."groupID" AND e."courseID"=cs."courseID" JOIN public.study_plan_periods p ON p."periodID"=sel."periodID" WHERE e."examStartAt">=public.study_term_start(p."academicYear",p.semester) AND e."examStartAt"<public.study_term_end(p."academicYear",p.semester);
CREATE VIEW public.student_course_teacher_access AS
 SELECT st."studentID",cg."courseID",ct."teacherID" FROM public.students st JOIN public."courseGroup" cg ON cg."groupID"=st."groupID" JOIN public."courseTeacher" ct ON ct."courseID"=cg."courseID" WHERE NOT EXISTS(SELECT 1 FROM public.student_stream_selections sel WHERE sel."studentID"=st."studentID" AND sel."courseID"=cg."courseID")
 UNION
 SELECT sel."studentID",sel."courseID",cs."teacherID" FROM public.student_stream_selections sel JOIN public.course_streams cs ON cs."streamID"=sel."streamID"
 UNION
 SELECT st."studentID",r."courseID",ct."teacherID" FROM public.study_plan_periods p JOIN public.study_plan_requirements r ON r."periodID"=p."periodID" AND r.kind='physical_education' JOIN public.students st ON st."facultyID"=p."facultyID" JOIN public."courseTeacher" ct ON ct."courseID"=r."courseID" WHERE p."isPublished" AND p."academicYear"=extract(year FROM st."studentEnrollmentDate")::integer+(p.semester-1)/2;
CREATE VIEW public.physical_course_scores AS
 SELECT md5(st."studentID"::text||p."periodID"::text||r."courseID"::text)::uuid AS "courseGraduate",r."courseID",st."studentID",
 (SELECT a."teacherID" FROM public.sport_attendance a WHERE a."studentID"=st."studentID" AND a.present AND a."attendedAt">=public.study_term_start(p."academicYear",p.semester) AND a."attendedAt"<public.study_term_end(p."academicYear",p.semester) ORDER BY a."updatedAt" DESC,a."attendanceID" LIMIT 1) AS "teacherID",
 least(stats.visits*5,100)::integer AS score,(stats.visits*5>=100) AS credit,
 COALESCE(stats.first_mark,public.study_term_start(p."academicYear",p.semester)) AS created_at,COALESCE(stats.last_mark,public.study_term_start(p."academicYear",p.semester)) AS updated_at
 FROM public.study_plan_periods p JOIN public.study_plan_requirements r ON r."periodID"=p."periodID" AND r.kind='physical_education' JOIN public.students st ON st."facultyID"=p."facultyID"
 CROSS JOIN LATERAL(SELECT count(*)::integer AS visits,min(a."markedAt") AS first_mark,max(a."updatedAt") AS last_mark FROM public.sport_attendance a WHERE a."studentID"=st."studentID" AND a.present AND a."attendedAt">=public.study_term_start(p."academicYear",p.semester) AND a."attendedAt"<public.study_term_end(p."academicYear",p.semester)) stats
 WHERE p."isPublished" AND p."academicYear"=extract(year FROM st."studentEnrollmentDate")::integer+(p.semester-1)/2 AND public.study_term_start(p."academicYear",p.semester)<=now();
CREATE VIEW public.effective_course_scores AS
 SELECT g.* FROM public."courseScore" g WHERE NOT EXISTS(SELECT 1 FROM public.study_plan_requirements r JOIN public.study_plan_periods p ON p."periodID"=r."periodID" JOIN public.students st ON st."studentID"=g."studentID" WHERE r."courseID"=g."courseID" AND r.kind='physical_education' AND p."isPublished" AND st."facultyID"=p."facultyID" AND p."academicYear"=extract(year FROM st."studentEnrollmentDate")::integer+(p.semester-1)/2)
 UNION ALL
 SELECT * FROM public.physical_course_scores;
DO $security$
DECLARE object_name text;role_name text;
BEGIN
 FOREACH object_name IN ARRAY ARRAY['study_plan_periods','study_plan_requirements','course_streams','student_stream_selections','sport_instructors','sport_attendance','sport_attendance_history'] LOOP
  EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY',object_name);
  EXECUTE format('REVOKE ALL ON TABLE public.%I FROM PUBLIC',object_name);
 END LOOP;
 FOREACH object_name IN ARRAY ARRAY['student_course_access','student_lesson_access','student_exam_access','student_course_teacher_access','physical_course_scores','effective_course_scores'] LOOP
  EXECUTE format('REVOKE ALL ON TABLE public.%I FROM PUBLIC',object_name);
 END LOOP;
 FOREACH role_name IN ARRAY ARRAY['anon','authenticated'] LOOP
  IF EXISTS(SELECT 1 FROM pg_catalog.pg_roles WHERE rolname=role_name) THEN
   FOREACH object_name IN ARRAY ARRAY['study_plan_periods','study_plan_requirements','course_streams','student_stream_selections','sport_instructors','sport_attendance','sport_attendance_history','student_course_access','student_lesson_access','student_exam_access','student_course_teacher_access','physical_course_scores','effective_course_scores'] LOOP
    EXECUTE format('REVOKE ALL ON TABLE public.%I FROM %I',object_name,role_name);
   END LOOP;
   EXECUTE format('REVOKE ALL ON SEQUENCE public.study_group_id_seq,public.study_lesson_id_seq FROM %I',role_name);
  END IF;
 END LOOP;
END;
$security$;
REVOKE ALL ON SEQUENCE public.study_group_id_seq,public.study_lesson_id_seq FROM PUBLIC;
