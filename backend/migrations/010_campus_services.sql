CREATE TABLE public.campus_benefits (
    id uuid PRIMARY KEY,
    title text NOT NULL CHECK (char_length(btrim(title)) BETWEEN 1 AND 120),
    description text NOT NULL CHECK (char_length(btrim(description)) BETWEEN 1 AND 2000),
    url text CHECK (url IS NULL OR char_length(url) <= 1000),
    published_by uuid REFERENCES public.accounts("accountID") ON UPDATE CASCADE ON DELETE SET NULL,
    published_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz
);

CREATE TABLE public.campus_lost_reports (
    id uuid PRIMARY KEY,
    student_id uuid NOT NULL REFERENCES public.students("studentID") ON UPDATE CASCADE ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('lost', 'found')),
    title text NOT NULL CHECK (char_length(btrim(title)) BETWEEN 1 AND 120),
    description text NOT NULL CHECK (char_length(btrim(description)) BETWEEN 1 AND 2000),
    location text NOT NULL CHECK (char_length(btrim(location)) BETWEEN 1 AND 180),
    created_at timestamptz NOT NULL DEFAULT now(),
    resolved_at timestamptz,
    resolution text CHECK (resolution IS NULL OR char_length(btrim(resolution)) BETWEEN 1 AND 2000),
    resolved_by uuid REFERENCES public.accounts("accountID") ON UPDATE CASCADE ON DELETE SET NULL,
    CHECK ((resolved_at IS NULL) = (resolution IS NULL))
);
CREATE INDEX campus_lost_reports_student ON public.campus_lost_reports (student_id, created_at DESC);

CREATE TABLE public.campus_resources (
    id uuid PRIMARY KEY,
    kind text NOT NULL CHECK (kind IN ('room', 'equipment', 'printing')),
    title text NOT NULL CHECK (char_length(btrim(title)) BETWEEN 1 AND 120),
    location text NOT NULL CHECK (char_length(btrim(location)) BETWEEN 1 AND 180)
);
CREATE UNIQUE INDEX campus_resource_identity ON public.campus_resources (lower(title), lower(location));

CREATE TABLE public.campus_resource_slots (
    id uuid PRIMARY KEY,
    resource_id uuid NOT NULL REFERENCES public.campus_resources(id) ON UPDATE CASCADE ON DELETE CASCADE,
    starts_at timestamptz NOT NULL,
    ends_at timestamptz NOT NULL CHECK (ends_at > starts_at),
    reserved_by uuid REFERENCES public.students("studentID") ON UPDATE CASCADE ON DELETE SET NULL
);
CREATE INDEX campus_resource_slots_resource ON public.campus_resource_slots (resource_id, starts_at);
CREATE INDEX campus_resource_slots_student ON public.campus_resource_slots (reserved_by, starts_at);

DO $security$
DECLARE
    table_name text;
    role_name text;
BEGIN
    FOREACH table_name IN ARRAY ARRAY['campus_benefits','campus_lost_reports','campus_resources','campus_resource_slots'] LOOP
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
