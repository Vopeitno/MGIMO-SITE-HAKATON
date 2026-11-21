# Подготовка Supabase и MongoDB Atlas

Секретные параметры заполняются в `backend/.env`, который уже исключён из Git и закрыт правами 0600. В текущем файле также записаны все шесть существующих демонстрационных аккаунтов: `DEMO_STUDENT1_*`, `DEMO_STUDENT2_*`, `DEMO_TEACHER_*`, `DEMO_SUPPORT_*`, `DEMO_DEAN_*`, `DEMO_ADMIN_*`.

Поля аккаунтов — справочник реально созданных записей. Редактирование `DEMO_*_PASSWORD` в файле не меняет хеш пароля уже существующего аккаунта в базе. При первом заполнении новой базы текущий seed использует общий `DEMO_PASSWORD` для всех шести аккаунтов.

## Supabase

1. Создать отдельный проект `mgimo-deanery` в организации с тарифом **Free**. Задать и сохранить пароль базы. Выбрать доступный европейский регион, например Frankfurt. Проверить, что создание остаётся на бесплатном тарифе. [Тарифы](https://supabase.com/pricing).
2. В созданном проекте открыть **Connect**, выбрать **Session pooler** и скопировать строку PostgreSQL на порту **5432**. Этот режим подходит для наших миграций с блокировкой сессии и для подключения по IPv4. [Подключения Supabase](https://supabase.com/docs/guides/database/connecting-to-postgres).
3. Вставить строку с настоящим паролем базы в существующее поле `DATABASE_URL`:

```dotenv
DATABASE_URL='postgresql://postgres.PROJECT_REF:URL_ENCODED_PASSWORD@POOLER_HOST:5432/postgres?sslmode=verify-full'
```

`POOLER_HOST` копируется из панели: его нельзя надёжно составить по названию региона. Специальные символы пароля в URI должны быть закодированы. Таблицы, связи и демонстрационные аккаунты добавит команда миграции/seed. Пользователей приложения вручную в Supabase Auth создавать не требуется: вход реализован сервером через таблицу `accounts`.

## MongoDB Atlas

1. Создать проект `mgimo-deanery`, в нём кластер `mgimo-chat` с тарифом **Free / M0**. Выбрать доступный европейский регион. [Создание бесплатного кластера](https://www.mongodb.com/docs/atlas/tutorial/deploy-free-tier-cluster/).
2. В **Database Access** создать пользователя базы `mgimo_backend`, сохранить его пароль и выдать роль `readWrite` для базы `mgimo`. [Пользователи базы](https://www.mongodb.com/docs/atlas/security-add-mongodb-users/).
3. В **Network Access** добавить текущий внешний IP компьютера через **Add Current IP Address**. После выбора хостинга потребуется разрешить внешний IP сервера. [Список разрешённых IP](https://www.mongodb.com/docs/atlas/security/ip-access-list/).
4. Открыть **Connect → Drivers → Go**, скопировать строку `mongodb+srv://…`, подставить имя пользователя и пароль. В `backend/.env` заменить:

```dotenv
MONGODB_URI='mongodb+srv://mgimo_backend:URL_ENCODED_PASSWORD@CLUSTER_HOST/?retryWrites=true&w=majority'
MONGODB_DATABASE=mgimo
```

Строка URI в примере заключена в кавычки, чтобы `&` корректно обрабатывался при загрузке `.env` через shell. [Строка подключения Atlas](https://www.mongodb.com/docs/atlas/driver-connection/). Коллекции и индексы создаются сервером.

## Что сообщить после заполнения

Достаточно написать: «Supabase и MongoDB созданы, `backend/.env` заполнен», а также указать выбранные регионы. Пароли в переписку присылать не требуется. Если трудно собрать строку, можно сообщить адрес проекта/кластера и имя пользователя, сохранив пароли в локальном `.env`.

После этого можно применить миграции к новой базе, добавить вымышленные данные, запустить Go с облачными подключениями и проверить права/сценарии. Нынешние `APP_ORIGIN=http://localhost:5173` и `COOKIE_SECURE=false` остаются настройками локального интерфейса. Их замена на публичный HTTPS-адрес относится к последующему развёртыванию.
