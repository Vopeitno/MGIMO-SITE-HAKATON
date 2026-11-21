CREATE TABLE public.campus_event_hosts (
    "eventID" uuid PRIMARY KEY REFERENCES public.campus_events("eventID") ON UPDATE CASCADE ON DELETE CASCADE,
    "teacherID" uuid NOT NULL REFERENCES public.teachers("teacherID") ON UPDATE CASCADE ON DELETE RESTRICT
);
CREATE INDEX campus_event_hosts_teacher_idx ON public.campus_event_hosts("teacherID");
ALTER TABLE public.campus_event_hosts ENABLE ROW LEVEL SECURITY;
REVOKE ALL ON public.campus_event_hosts FROM PUBLIC;
DO $security$
DECLARE role_name text;
BEGIN
    FOREACH role_name IN ARRAY ARRAY['anon','authenticated'] LOOP
        IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname=role_name) THEN
            EXECUTE format('REVOKE ALL ON public.campus_event_hosts FROM %I',role_name);
        END IF;
    END LOOP;
END;
$security$;

ALTER TABLE public.campus_event_registrations
    ADD COLUMN "ticketID" uuid NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    ADD COLUMN "checkedByAccountID" uuid REFERENCES public.accounts("accountID") ON UPDATE CASCADE ON DELETE SET NULL,
    ADD COLUMN "checkMethod" text CHECK ("checkMethod" IN ('qr','support'));
