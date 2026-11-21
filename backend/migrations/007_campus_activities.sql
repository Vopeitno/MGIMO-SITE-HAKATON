CREATE TABLE public.campus_events (
    "eventID" uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    "facultyID" uuid NOT NULL REFERENCES public.faculties("facultyID") ON UPDATE CASCADE ON DELETE CASCADE,
    "buildingID" uuid NOT NULL REFERENCES public.buildings("buildingID") ON UPDATE CASCADE ON DELETE RESTRICT,
    "createdByEmployerID" uuid REFERENCES public.employers("employerID") ON UPDATE CASCADE ON DELETE SET NULL,
    title varchar(128) NOT NULL CHECK (btrim(title)<>''),
    description varchar(2000) NOT NULL,
    category varchar(16) NOT NULL CHECK (category IN ('community','career','volunteering')),
    "startAt" timestamptz NOT NULL,
    "endAt" timestamptz NOT NULL CHECK ("endAt">"startAt"),
    capacity integer NOT NULL CHECK (capacity BETWEEN 1 AND 10000),
    "createdAt" timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX campus_events_start_idx ON public.campus_events("startAt", "eventID");

CREATE TABLE public.campus_event_registrations (
    "eventID" uuid NOT NULL REFERENCES public.campus_events("eventID") ON UPDATE CASCADE ON DELETE CASCADE,
    "studentID" uuid NOT NULL REFERENCES public.students("studentID") ON UPDATE CASCADE ON DELETE CASCADE,
    "registeredAt" timestamptz NOT NULL DEFAULT now(),
    "canceledAt" timestamptz,
    "attendedAt" timestamptz,
    CHECK ("canceledAt" IS NULL OR "attendedAt" IS NULL),
    PRIMARY KEY ("eventID","studentID")
);
CREATE INDEX campus_registrations_student_idx ON public.campus_event_registrations("studentID","eventID");

DO $security$
DECLARE table_name text; role_name text;
BEGIN
    FOREACH table_name IN ARRAY ARRAY['campus_events','campus_event_registrations'] LOOP
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
