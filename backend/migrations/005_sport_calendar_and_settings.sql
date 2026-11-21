CREATE TABLE public.sport_session_rooms (
    "sportID" uuid PRIMARY KEY REFERENCES public.sports("sportID") ON UPDATE CASCADE ON DELETE CASCADE,
    room text NOT NULL CHECK (char_length(room) BETWEEN 1 AND 100 AND room = btrim(room)),
    room_key text NOT NULL
);

CREATE INDEX sport_session_rooms_key_idx ON public.sport_session_rooms(room_key);

CREATE TABLE public.student_notification_preferences (
    "studentID" uuid NOT NULL REFERENCES public.students("studentID") ON UPDATE CASCADE ON DELETE CASCADE,
    category public.notification_category NOT NULL,
    PRIMARY KEY ("studentID", category)
);

CREATE TABLE public.student_documents (
    "documentID" uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    title text NOT NULL CHECK (btrim(title) <> ''),
    version text NOT NULL CHECK (btrim(version) <> ''),
    body text NOT NULL CHECK (btrim(body) <> ''),
    "requiresConsent" boolean NOT NULL DEFAULT false,
    "publishedAt" timestamptz,
    UNIQUE (title, version)
);

CREATE TABLE public.student_document_choices (
    "choiceID" uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    "studentID" uuid NOT NULL REFERENCES public.students("studentID") ON UPDATE CASCADE ON DELETE CASCADE,
    "documentID" uuid NOT NULL REFERENCES public.student_documents("documentID") ON UPDATE CASCADE ON DELETE RESTRICT,
    accepted boolean NOT NULL,
    "changedAt" timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX student_document_choices_lookup_idx ON public.student_document_choices
    ("studentID", "documentID", "changedAt" DESC, "choiceID" DESC);

DO $security$
DECLARE
    table_name text;
    role_name text;
BEGIN
    FOREACH table_name IN ARRAY ARRAY['sport_session_rooms','student_notification_preferences','student_documents','student_document_choices'] LOOP
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
