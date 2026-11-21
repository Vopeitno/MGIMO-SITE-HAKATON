CREATE TABLE public.staff_account_audit (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id uuid REFERENCES public.accounts("accountID") ON UPDATE CASCADE ON DELETE SET NULL,
    target_id uuid REFERENCES public.accounts("accountID") ON UPDATE CASCADE ON DELETE SET NULL,
    target_email text NOT NULL,
    action text NOT NULL CHECK (action IN ('created','updated')),
    previous_role text,
    next_role text NOT NULL,
    previous_active boolean,
    next_active boolean NOT NULL,
    changed_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX staff_account_audit_date ON public.staff_account_audit(changed_at DESC,id);

CREATE TABLE public.staff_course_materials (
    id uuid PRIMARY KEY,
    course_group_id uuid NOT NULL REFERENCES public."courseGroup"("courseGroupID") ON UPDATE CASCADE ON DELETE CASCADE,
    teacher_id uuid NOT NULL REFERENCES public.teachers("teacherID") ON UPDATE CASCADE ON DELETE CASCADE,
    title text NOT NULL CHECK (char_length(btrim(title)) BETWEEN 1 AND 150),
    description text NOT NULL CHECK (char_length(btrim(description)) BETWEEN 1 AND 3000),
    url text CHECK (url IS NULL OR char_length(url)<=1000),
    file_name text,
    published boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (num_nonnulls(url,file_name)<=1),
    CHECK (published OR (url IS NULL AND file_name IS NULL))
);
CREATE INDEX staff_course_materials_group ON public.staff_course_materials(course_group_id,created_at DESC,id);
CREATE INDEX staff_course_materials_teacher ON public.staff_course_materials(teacher_id,created_at DESC,id);

DO $security$
DECLARE table_name text;
        role_name text;
BEGIN
    FOREACH table_name IN ARRAY ARRAY['staff_account_audit','staff_course_materials'] LOOP
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
