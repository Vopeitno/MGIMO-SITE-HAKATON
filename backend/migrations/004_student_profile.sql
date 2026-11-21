CREATE TABLE public.student_profile_photos (
    "studentID" uuid PRIMARY KEY REFERENCES public.students("studentID") ON UPDATE CASCADE ON DELETE CASCADE,
    jpeg bytea NOT NULL CHECK (octet_length(jpeg) BETWEEN 1 AND 524288),
    "updatedAt" timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE public.student_record_books (
    "studentID" uuid PRIMARY KEY REFERENCES public.students("studentID") ON UPDATE CASCADE ON DELETE CASCADE,
    "recordBookNumber" varchar(64) NOT NULL UNIQUE CHECK (btrim("recordBookNumber") = "recordBookNumber" AND "recordBookNumber" <> '')
);

ALTER TABLE public.student_profile_photos ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.student_record_books ENABLE ROW LEVEL SECURITY;
REVOKE ALL ON public.student_profile_photos, public.student_record_books FROM PUBLIC;
DO $security$
DECLARE
    role_name text;
BEGIN
    FOREACH role_name IN ARRAY ARRAY['anon','authenticated'] LOOP
        IF EXISTS(SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = role_name) THEN
            EXECUTE format('REVOKE ALL ON public.student_profile_photos, public.student_record_books FROM %I', role_name);
        END IF;
    END LOOP;
END;
$security$;
