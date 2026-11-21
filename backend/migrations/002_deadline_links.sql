CREATE TABLE public.deadline_links (
 "deadlineID" integer PRIMARY KEY REFERENCES public.deadlines("deadlineID") ON UPDATE CASCADE ON DELETE CASCADE,
 "submissionURL" varchar(2048) NOT NULL CHECK ("submissionURL" ~ '^https://[^[:space:]]+$'),
 "updatedByTeacherID" uuid REFERENCES public.teachers("teacherID") ON UPDATE CASCADE ON DELETE SET NULL,
 "updatedAt" timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX deadline_links_teacher_idx ON public.deadline_links("updatedByTeacherID");
ALTER TABLE public.deadline_links ENABLE ROW LEVEL SECURITY;
REVOKE ALL ON public.deadline_links FROM PUBLIC;
DO $security$
DECLARE role_name text;
BEGIN
 FOREACH role_name IN ARRAY ARRAY['anon','authenticated'] LOOP
  IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname=role_name) THEN
   EXECUTE format('REVOKE ALL ON TABLE public.deadline_links FROM %I',role_name);
  END IF;
 END LOOP;
END;
$security$;
