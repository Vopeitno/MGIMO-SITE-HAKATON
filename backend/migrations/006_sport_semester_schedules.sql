CREATE TABLE public.sport_semester_schedules (
    "scheduleID" uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    "facultyID" uuid NOT NULL REFERENCES public.faculties("facultyID") ON UPDATE CASCADE ON DELETE CASCADE,
    title varchar(128) NOT NULL,
    title_key text NOT NULL,
    "academicYear" integer NOT NULL CHECK ("academicYear" BETWEEN 2000 AND 2199),
    term integer NOT NULL CHECK (term IN (1,2)),
    "createdAt" timestamptz NOT NULL DEFAULT now(),
    UNIQUE ("facultyID", title_key, "academicYear", term)
);
CREATE TABLE public.sport_weekly_slots (
    "slotID" uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    "scheduleID" uuid NOT NULL REFERENCES public.sport_semester_schedules("scheduleID") ON UPDATE CASCADE ON DELETE CASCADE,
    weekday integer NOT NULL CHECK (weekday BETWEEN 1 AND 7),
    "startTime" time NOT NULL,
    "endTime" time NOT NULL CHECK ("endTime">"startTime"),
    "classroomID" integer NOT NULL REFERENCES public.classroom("classroomID") ON UPDATE CASCADE ON DELETE RESTRICT,
    "teacherID" uuid NOT NULL REFERENCES public.teachers("teacherID") ON UPDATE CASCADE ON DELETE RESTRICT,
    capacity integer NOT NULL CHECK (capacity BETWEEN 1 AND 10000)
);
CREATE INDEX sport_weekly_slots_schedule_idx ON public.sport_weekly_slots("scheduleID");
CREATE TABLE public.sport_schedule_occurrences (
    "sportID" uuid PRIMARY KEY REFERENCES public.sports("sportID") ON UPDATE CASCADE ON DELETE CASCADE,
    "slotID" uuid REFERENCES public.sport_weekly_slots("slotID") ON UPDATE CASCADE ON DELETE SET NULL
);

DO $security$
DECLARE
    table_name text;
    role_name text;
BEGIN
    FOREACH table_name IN ARRAY ARRAY['sport_semester_schedules','sport_weekly_slots','sport_schedule_occurrences'] LOOP
        EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY',table_name);
        EXECUTE format('REVOKE ALL ON public.%I FROM PUBLIC',table_name);
        FOREACH role_name IN ARRAY ARRAY['anon','authenticated'] LOOP
            IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname=role_name) THEN
                EXECUTE format('REVOKE ALL ON public.%I FROM %I',table_name,role_name);
            END IF;
        END LOOP;
    END LOOP;
END;
$security$;
