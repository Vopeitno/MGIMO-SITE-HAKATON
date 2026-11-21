INSERT INTO public.appointment_types ("appointmentTypeID", "appointmentTypeTitle", "appointmentTypeDescription")
SELECT 'd553e67f-0eb6-44b4-99f3-d826f1c31f9e', 'Материальная помощь',
       'Укажите причину обращения и при необходимости приложите документы.'
WHERE NOT EXISTS (
    SELECT 1 FROM public.appointment_types
    WHERE lower(trim("appointmentTypeTitle")) = 'материальная помощь'
)
ON CONFLICT ("appointmentTypeTitle") DO NOTHING;
