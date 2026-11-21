CREATE TABLE public.support_articles (
    id uuid PRIMARY KEY,
    question text NOT NULL CHECK (char_length(btrim(question)) BETWEEN 1 AND 180),
    answer text NOT NULL CHECK (char_length(btrim(answer)) BETWEEN 1 AND 5000),
    updated_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz
);

CREATE TABLE public.support_tickets (
    id uuid PRIMARY KEY,
    student_id uuid NOT NULL REFERENCES public.students("studentID") ON UPDATE CASCADE ON DELETE CASCADE,
    category text NOT NULL CHECK (category IN ('study', 'schedule', 'technical', 'dormitory', 'other', 'consultation')),
    priority text NOT NULL CHECK (priority IN ('normal', 'urgent')),
    assigned_to uuid REFERENCES public.accounts("accountID") ON UPDATE CASCADE ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    closed_at timestamptz
);
CREATE INDEX support_tickets_student ON public.support_tickets (student_id, updated_at DESC);
CREATE INDEX support_tickets_assignee ON public.support_tickets (assigned_to);

CREATE TABLE public.support_ticket_messages (
    id uuid PRIMARY KEY,
    ticket_id uuid NOT NULL REFERENCES public.support_tickets(id) ON UPDATE CASCADE ON DELETE CASCADE,
    from_student boolean NOT NULL,
    message text NOT NULL CHECK (char_length(btrim(message)) BETWEEN 1 AND 5000),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX support_ticket_messages_order ON public.support_ticket_messages (ticket_id, created_at, id);

INSERT INTO public.support_articles (id, question, answer) VALUES
    ('6fb05496-a769-40b0-b7d6-82af914e3bd1', 'Как получить справку об обучении?',
     'Откройте «Заявки», нажмите «Заказать справку» и выберите тип справки и способ получения. Статус можно посмотреть в истории заявок.'),
    ('6fb05496-a769-40b0-b7d6-82af914e3bd2', 'Что делать, если в расписании ошибка?',
     'Создайте обращение по теме «Расписание». Укажите дату, время и название занятия, чтобы сотрудник мог проверить данные.'),
    ('6fb05496-a769-40b0-b7d6-82af914e3bd3', 'Где найти QR-визитку студента?',
     'Откройте «Профиль» и нажмите «Показать QR-профиль». Для проходной требуется отдельный пропуск.');

DO $security$
DECLARE
    table_name text;
    role_name text;
BEGIN
    FOREACH table_name IN ARRAY ARRAY['support_articles','support_tickets','support_ticket_messages'] LOOP
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
