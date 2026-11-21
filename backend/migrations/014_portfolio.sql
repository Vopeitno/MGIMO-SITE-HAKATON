CREATE TABLE public.student_portfolios (
    student_id uuid PRIMARY KEY REFERENCES public.students("studentID") ON UPDATE CASCADE ON DELETE CASCADE,
    bio text NOT NULL DEFAULT '' CHECK (char_length(bio) <= 1500),
    share_id uuid UNIQUE
);

CREATE TABLE public.student_portfolio_items (
    id uuid PRIMARY KEY,
    student_id uuid NOT NULL REFERENCES public.students("studentID") ON UPDATE CASCADE ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('project','certificate','achievement','publication','volunteering','skill')),
    title text NOT NULL CHECK (char_length(btrim(title)) BETWEEN 1 AND 150),
    description text NOT NULL CHECK (char_length(btrim(description)) BETWEEN 1 AND 3000),
    url text CHECK (url IS NULL OR char_length(url) <= 1000),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX student_portfolio_items_student ON public.student_portfolio_items (student_id, created_at DESC, id);

DO $security$
DECLARE
    table_name text;
    role_name text;
BEGIN
    FOREACH table_name IN ARRAY ARRAY['student_portfolios','student_portfolio_items'] LOOP
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
