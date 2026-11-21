# Начальная миграция PostgreSQL

`001_initial.sql` создаёт схему из `docs/database.dbml` в `public` одной транзакцией. Применять только к пустой схеме; повторный запуск намеренно завершится ошибкой. Автоматического применения и обращения к облаку нет.

Имена заключены в двойные кавычки: регистр, `appoinmentID`, `departament`, `passportCodeDivison` и остальные исходные написания сохранены. По согласованию все поля `passport*` имеют тип `varchar(256)` для хешей Argon2id; размеры синхронизированы в DBML и встроенной DBML в requirements.md. Паспортные поля не должны возвращаться через API. Эта миграция не реализует хеширование и фильтрацию ответов: это обязанности серверной части. UUID и целочисленные идентификаторы не имеют непредписанных генераторов. Все DEFAULT, ENUM, PRIMARY KEY, UNIQUE, NOT NULL и CHECK перенесены из DBML.

Для всех внешних ключей установлено `ON UPDATE CASCADE`. Индексы внешних ключей созданы отдельно, кроме случаев, когда столбец уже первый в индексе PRIMARY KEY или UNIQUE.

## Карта удаления

Действие относится к удалению строки из родительской таблицы. RESTRICT запрещает удаление при наличии ссылки; SET NULL сохраняет зависимую строку; CASCADE удаляет её.

| Ссылка | Родитель | ON DELETE |
| --- | --- | --- |
| `exams.courseID` | `courses.courseID` | `RESTRICT` |
| `exams.groupID` | `groups.groupID` | `RESTRICT` |
| `exams.teacherID` | `teachers.teacherID` | `RESTRICT` |
| `exams.classroomID` | `classroom.classroomID` | `SET NULL` |
| `exam_results.examID` | `exams.examID` | `RESTRICT` |
| `exam_results.studentID` | `students.studentID` | `RESTRICT` |
| `sports.buildingID` | `buildings.buildingID` | `SET NULL` |
| `sport_registrations.sportID` | `sports.sportID` | `CASCADE` |
| `sport_registrations.studentID` | `students.studentID` | `CASCADE` |
| `appointment.appointmentTypeID` | `appointment_types.appointmentTypeID` | `SET NULL` |
| `appointment_drafts.studentID` | `students.studentID` | `RESTRICT` |
| `appointment_drafts.appointmentTypeID` | `appointment_types.appointmentTypeID` | `SET NULL` |
| `courseGroup.courseID` | `courses.courseID` | `CASCADE` |
| `courseTeacher.teacherID` | `teachers.teacherID` | `CASCADE` |
| `courseTeacher.courseID` | `courses.courseID` | `CASCADE` |
| `facultyTeacher.facultyID` | `faculties.facultyID` | `CASCADE` |
| `facultyTeacher.teacherID` | `teachers.teacherID` | `CASCADE` |
| `direction.facultyID` | `faculties.facultyID` | `RESTRICT` |
| `students.groupID` | `groups.groupID` | `RESTRICT` |
| `courseGroup.groupID` | `groups.groupID` | `CASCADE` |
| `lessons.classroomID` | `classroom.classroomID` | `RESTRICT` |
| `lessons.groupID` | `groups.groupID` | `RESTRICT` |
| `lessons.teacherID` | `teachers.teacherID` | `RESTRICT` |
| `lesson_changes.lessonID` | `lessons.lessonID` | `SET NULL` |
| `lesson_changes.oldRoomID` | `classroom.classroomID` | `SET NULL` |
| `lesson_changes.newRoomID` | `classroom.classroomID` | `SET NULL` |
| `lesson_changes.oldTeacherID` | `teachers.teacherID` | `SET NULL` |
| `lesson_changes.newTeacherID` | `teachers.teacherID` | `SET NULL` |
| `lesson_changes.changedByTeacherID` | `teachers.teacherID` | `SET NULL` |
| `courseScore.studentID` | `students.studentID` | `RESTRICT` |
| `courseScore.courseID` | `courses.courseID` | `RESTRICT` |
| `courseScore.teacherID` | `teachers.teacherID` | `RESTRICT` |
| `appointment.studentID` | `students.studentID` | `RESTRICT` |
| `appointment_history.appointmentID` | `appointment.appoinmentID` | `RESTRICT` |
| `departament.directorID` | `director.directorID` | `RESTRICT` |
| `employers.departamentID` | `departament.departamentID` | `SET NULL` |
| `appointment.employerID` | `employers.employerID` | `SET NULL` |
| `deaneries.deanID` | `employers.employerID` | `SET NULL` |
| `news.employerID` | `employers.employerID` | `SET NULL` |
| `news.deaneryID` | `deaneries.deaneryID` | `SET NULL` |
| `students.facultyID` | `faculties.facultyID` | `SET NULL` |
| `faculties.deaneryID` | `deaneries.deaneryID` | `RESTRICT` |
| `deadlines.courseID` | `courses.courseID` | `SET NULL` |
| `classroom.buildingID` | `buildings.buildingID` | `RESTRICT` |
| `appointment.buildingID` | `buildings.buildingID` | `SET NULL` |
| `notifications.studentID` | `students.studentID` | `CASCADE` |
| `notifications.employerID` | `employers.employerID` | `CASCADE` |
| `courses.facultyID` | `faculties.facultyID` | `RESTRICT` |
| `students.directionID` | `direction.directionID` | `SET NULL` |
| `lessons.courseID` | `courses.courseID` | `RESTRICT` |
| `appointment.teacherID` | `teachers.teacherID` | `RESTRICT` |
| `appointment.createdByEmployerID` | `employers.employerID` | `RESTRICT` |
| `appointment.subjectStudentID` | `students.studentID` | `SET NULL` |
| `appointment_drafts.teacherID` | `teachers.teacherID` | `RESTRICT` |
| `appointment_drafts.createdByEmployerID` | `employers.employerID` | `RESTRICT` |
| `notifications.teacherID` | `teachers.teacherID` | `CASCADE` |
| `notifications.lessonID` | `lessons.lessonID` | `CASCADE` |
| `notifications.appoinmentID` | `appointment.appoinmentID` | `CASCADE` |
| `notifications.courseID` | `courses.courseID` | `CASCADE` |
| `notifications.examID` | `exams.examID` | `CASCADE` |
| `notifications.sportID` | `sports.sportID` | `CASCADE` |
| `notifications.newsID` | `news.newsID` | `CASCADE` |
| `lesson_changes.changedByEmployerID` | `employers.employerID` | `SET NULL` |
| `appointment_history.changedByAccountID` | `accounts.accountID` | `SET NULL` |
| `accounts.studentID` | `students.studentID` | `CASCADE` |
| `accounts.teacherID` | `teachers.teacherID` | `CASCADE` |
| `accounts.employerID` | `employers.employerID` | `CASCADE` |
| `user_sessions.accountID` | `accounts.accountID` | `CASCADE` |
| `deanery_staff.deaneryID` | `deaneries.deaneryID` | `CASCADE` |
| `deanery_staff.employerID` | `employers.employerID` | `CASCADE` |
| `appointment_assignments.appointmentID` | `appointment.appoinmentID` | `RESTRICT` |
| `appointment_assignments.oldEmployerID` | `employers.employerID` | `SET NULL` |
| `appointment_assignments.newEmployerID` | `employers.employerID` | `RESTRICT` |
| `appointment_assignments.changedByAccountID` | `accounts.accountID` | `RESTRICT` |

Связующие записи, спортивные регистрации, учётные записи, их сессии и уведомления удаляются каскадно. Удаление человека удаляет его учётную запись, но может быть заблокировано обязательными ссылками на неё из истории назначений. Авторы заявок и черновиков защищены RESTRICT, несмотря на nullable столбцы: SET NULL нарушил бы CHECK единственного автора. Получатели уведомлений удаляются вместе с уведомлением, поэтому CHECK единственного получателя сохраняется.

История заявок и назначений защищена RESTRICT при удалении заявки. Обязательные учебные ссылки, результаты экзаменов и оценки защищены RESTRICT; nullable исторические и контекстные ссылки используют SET NULL. Это позволяет сохранять историю и не вводить условных триггеров удаления. Таблица courseGroup трактуется как связующая.

## Время и доступ

Перенесены только предписанные проверки времени: `exam_valid_time`, `sport_valid_time`, `session_valid_expiration`. Дополнительные проверки порядка времён занятий, курсов, дедлайнов и изменения записей не добавлены: они изменили бы допустимые данные относительно DBML.

На всех таблицах включён RLS, публичных политик нет. Права на эти таблицы отозваны у PUBLIC и, если такие роли существуют, у anon/authenticated. Создание ролей и изменение прав других объектов не выполняется. Владелец таблиц и роли с BYPASSRLS обходят RLS; серверное подключение и его полномочия настраиваются отдельно. Миграция не создаёт серверную роль и не выдаёт доступ браузеру.

## Выполненная проверка

Миграция выполнена в двух изолированных локальных базах PostgreSQL 18: без ролей Supabase и с существующими anon/authenticated и заранее выданными через DEFAULT PRIVILEGES правами. Облачная база не использовалась.

Программно сверены DBML и системный каталог: 33 таблицы, 252 колонки (имена, порядок, типы, NULL и DEFAULT), 11 перечислений с порядком значений, PRIMARY KEY/UNIQUE, 12 CHECK (исходные выражения дополнительно сверены с SQL), 74 внешних ключа с действиями обновления/удаления, покрытие индексами, RLS и отсутствие публичных политик/прав. Размеры паспортных полей проверены в обеих DBML-схемах.

Проверены фактические сценарии: запрет удаления автора заявки и черновика, каскадное обновление идентификатора преподавателя, каскадное удаление учётной записи/сессии/уведомления, сохранение истории занятия с SET NULL, отклонение уведомления без получателя и неверного срока сессии, отказ в чтении для anon/authenticated. Проверочные данные откатились; временный сервер остановлен.


## 002_deadline_links.sql — ссылки преподавателя на задания

Новая отдельная таблица, без изменения прежних сущностей. deadlineID — PK/FK, ON UPDATE/DELETE CASCADE. updatedByTeacherID — FK, ON UPDATE CASCADE/DELETE SET NULL; удаление преподавателя сохраняет ссылку и обнуляет автора. RLS включена, доступ PUBLIC/anon/authenticated отозван. Запись только через проверяющий роль и назначение преподавателя Go API.

## 010 — сервисы кампуса

Добавляет каталог льгот, закрытый реестр бюро находок, справочник ресурсов и слоты. Для всех четырёх таблиц включена RLS и отозван прямой доступ ролей PUBLIC, anon и authenticated. Действующие таблицы не меняются. При удалении студента его сообщения удаляются, а зарезервированные им слоты освобождаются.

## 011 — поддержка студентов

Создаёт базу знаний, обращения и переписку по обращениям. Для новых таблиц включена RLS и отозван прямой доступ ролей PUBLIC, anon и authenticated. Миграция публикует три стартовые статьи о существующих функциях портала.

## 012 — карточки вещей и расширение базы знаний

Каталог вещей доступен авторизованным студентам через API. Для записи требуется роль поддержки. Существующие обращения получают необязательную связь с карточкой. В историю базы знаний добавляются ещё 12 инструкций.

## 013 — карьерные предложения и практика

Добавляет предложения факультетов и назначения практики. В базе действует уникальность незавершённой практики студента. Файлы отчётов обслуживаются авторизованным API через общее закрытое хранилище вложений. Для обеих таблиц включена RLS, прямой доступ публичных ролей отозван.

## 014 — портфолио

Добавляет описание студента, отзывную публичную ссылку и карточки портфолио. Собственные данные редактируются через авторизованный API. В обеих таблицах включена RLS и отозван прямой доступ публичных ролей.

## 015 — кабинеты сотрудников

Добавляет материалы учебных групп и журнал изменений учётных записей. Запись в журнал создаётся в одной транзакции с изменением аккаунта. Файлы материалов обслуживает закрытое хранилище, а список студентов ограничен группой или выбранным потоком. Для обеих новых таблиц включена RLS.
