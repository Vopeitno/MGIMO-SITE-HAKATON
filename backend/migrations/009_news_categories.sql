CREATE TABLE public.news_categories (
    "newsID" integer PRIMARY KEY REFERENCES public.news("newsID") ON UPDATE CASCADE ON DELETE CASCADE,
    category text NOT NULL CHECK (category IN ('important','opportunities','campus','events'))
);
ALTER TABLE public.news_categories ENABLE ROW LEVEL SECURITY;
REVOKE ALL ON public.news_categories FROM PUBLIC;
DO $security$
DECLARE role_name text;
BEGIN
    FOREACH role_name IN ARRAY ARRAY['anon','authenticated'] LOOP
        IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname=role_name) THEN
            EXECUTE format('REVOKE ALL ON public.news_categories FROM %I', role_name);
        END IF;
    END LOOP;
END;
$security$;
