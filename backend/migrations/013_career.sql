CREATE TABLE public.career_offers (
    id uuid PRIMARY KEY,
    faculty_id uuid NOT NULL REFERENCES public.faculties("facultyID") ON UPDATE CASCADE ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('vacancy','internship')),
    title text NOT NULL CHECK (char_length(btrim(title)) BETWEEN 1 AND 150),
    organization text NOT NULL CHECK (char_length(btrim(organization)) BETWEEN 1 AND 160),
    location text NOT NULL CHECK (char_length(btrim(location)) BETWEEN 1 AND 120),
    description text NOT NULL CHECK (char_length(btrim(description)) BETWEEN 1 AND 3000),
    url text NOT NULL CHECK (char_length(url) BETWEEN 1 AND 1000),
    published_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz
);
CREATE INDEX career_offers_faculty ON public.career_offers (faculty_id, published_at DESC) WHERE archived_at IS NULL;

CREATE TABLE public.career_practices (
    id uuid PRIMARY KEY,
    student_id uuid NOT NULL REFERENCES public.students("studentID") ON UPDATE CASCADE ON DELETE CASCADE,
    organization text NOT NULL CHECK (char_length(btrim(organization)) BETWEEN 1 AND 160),
    starts_on date NOT NULL,
    ends_on date NOT NULL CHECK (ends_on >= starts_on),
    report_due_on date NOT NULL CHECK (report_due_on >= ends_on),
    state text NOT NULL DEFAULT 'assigned' CHECK (state IN ('assigned','submitted','revision','accepted')),
    report_file text,
    feedback text,
    CHECK ((report_file IS NULL) = (state = 'assigned'))
);
CREATE UNIQUE INDEX career_one_active_practice ON public.career_practices (student_id) WHERE state <> 'accepted';
CREATE INDEX career_practice_student ON public.career_practices (student_id, starts_on DESC);

DO $security$
DECLARE table_name text;
        role_name text;
BEGIN
    FOREACH table_name IN ARRAY ARRAY['career_offers','career_practices'] LOOP
        EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY', table_name);
        EXECUTE format('REVOKE ALL ON public.%I FROM PUBLIC', table_name);
        FOREACH role_name IN ARRAY ARRAY['anon','authenticated'] LOOP
            IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname=role_name) THEN
                EXECUTE format('REVOKE ALL ON public.%I FROM %I', table_name, role_name);
            END IF;
        END LOOP;
    END LOOP;
END;
$security$;
