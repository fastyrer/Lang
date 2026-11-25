# Lingua

Стартовая версия школы английского и китайского. React отвечает за главную и личный кабинет, Go — за аккаунты, каталог, запись и учёт абонементов.

## Запуск через Docker

Нужен запущенный Docker Engine или Docker Desktop с Docker Compose. Из корня проекта выполните одну команду:

```sh
./start.sh
```

Скрипт собирает React и Go внутри Docker, запускает контейнер и ждёт успешной проверки готовности. Сайт откроется на [http://127.0.0.1:8081](http://127.0.0.1:8081).

```sh
docker compose logs -f app  # Смотреть логи
docker compose down         # Остановить контейнер и сеть
```

Профили, записи, абонементы и сессии хранятся в томе `lingua_lingua-data`. Том сохраняется при обновлении образа, пересоздании контейнера и остановке через `docker compose down`. Чтобы применить изменения в коде, снова запустите `./start.sh`.

Порт по умолчанию доступен только с этого компьютера. Параметры можно указать перед командой, например `LINGUA_PORT=8090 ./start.sh`, или записать в `.env` в корне проекта.

| Переменная | По умолчанию | Назначение |
| --- | --- | --- |
| `LINGUA_HOST` | `127.0.0.1` | Интерфейс, на котором Docker публикует порт |
| `LINGUA_PORT` | `8081` | Порт на компьютере с Docker |
| `LINGUA_SECURE_COOKIES` | `0` | Установите `1` при доступе через HTTPS-прокси |

### Расписание в Docker

Сохраните расписание преподавателя в `schedule.json` в корне проекта и создайте рядом `compose.override.yaml`. Образ получит расписание через монтирование файла только для чтения.

```yaml
services:
  app:
    environment:
      LINGUA_SCHEDULE: /app/schedule.json
    volumes:
      - type: bind
        source: ./schedule.json
        target: /app/schedule.json
        read_only: true
        bind:
          create_host_path: false
```

После добавления файла выполните `./start.sh`. Если позже измените расписание, перезапустите контейнер командой `docker compose restart app`. Формат `schedule.json` описан ниже.

## Локальный запуск без Docker

Нужны Go 1.25+ и Node.js 20+ с npm. Из корня проекта:

```sh
npm --prefix web ci
npm --prefix web run build
go run ./cmd/lingua
```

Сайт откроется на [http://127.0.0.1:8080](http://127.0.0.1:8080). Сборку React и API раздаёт один сервер. Для разработки с автообновлением оставьте Go-сервер включённым и запустите в другом терминале `npm --prefix web run dev`. Vite перенаправит запросы API на Go.

Проверка Go: `go test -race ./...`.

## Программа и запись

- Откройте пример модуля без регистрации. В программе видны учебник, глава, уровень, требуемые знания и результат.
- В календаре выберите язык, формат, модуль, дату и время. Для мини-групп видны тема, уровень и количество мест. Часовой пояс расписания — Москва.
- В кабинете можно перенести урок, добавить его в календарь через `.ics`, открыть видеовстречу и отметить самостоятельную подготовку. Срок переноса и отмены указан при записи. Для новых записей по умолчанию действует правило 8 часов до начала.
- В разделе «Моё обучение» видны подготовка к ближайшему занятию, отметки ученика, комментарий преподавателя и рекомендованный модуль.
- Пакет даёт четыре занятия на 30 дней. На локальном запуске цены демонстрационные. Платный пакет оформляется разово, без автопродления, когда настроена оплата.

## Кабинет преподавателя

Создайте файл `.env.lingua.local` рядом с `compose.yaml`. Укажите `LINGUA_TEACHER_EMAIL` и `LINGUA_TEACHER_PASSWORD` (от 12 символов) либо `LINGUA_TEACHER_PASSWORD_FILE` с путём к секрету внутри контейнера. Настройка создаёт один аккаунт преподавателя при первом запуске. После запуска пароль можно убрать из конфигурации. Обычный ученический аккаунт с тем же email автоматически не повышается до преподавателя.

Для управления запустите `./start.sh`, войдите и откройте «Преподаватель». Там можно открыть время, задать тему и число мест, изменить ссылку или отменить занятие. Подтверждённые записи защищают время преподавателя от пересечений. Первое созданное в кабинете занятие заменяет демо-расписание. После занятия можно сохранить отзыв, отметить модуль пройденным и назначить следующий.

## Учебники, профиль и контакты

Без конфигурации показываются подписанные примеры программ. Для реального профиля создайте `school.json` по образцу [deploy/school.example.json](deploy/school.example.json). Замените имя, описания, книги, модули, цены и ссылки. Установите `"sample": false`, когда программа готова. Идентификаторы книг и модулей должны оставаться стабильными после появления ученических записей. Добавляйте главы и ссылки на разрешённые к использованию материалы. Поле `open` определяет, видна ли подготовка гостю. Фото можно положить в `web/public/images/` и указать путь вида `/images/teacher.webp`.

В `compose.override.yaml` добавьте монтирование и переменную (объедините с уже настроенным расписанием, если оно есть):

```yaml
services:
  app:
    environment:
      LINGUA_SCHOOL: /app/school.json
    volumes:
      - ./school.json:/app/school.json:ro
```

Запустите `./start.sh`. После изменения содержимого `school.json` перезапустите приложение командой `docker compose restart app`.

## Публичный запуск, почта и оплата

1. На сервере с Docker направьте DNS домена на сервер, откройте порты 80 и 443 и задайте `LINGUA_DOMAIN` в `.env`. Запуск через Caddy с HTTPS и защищёнными cookies: `LINGUA_DEPLOY_HTTPS=1 ./start.sh`. В этом режиме приложение доступно через прокси, а демо-активация отключена.
2. Заполните в `.env.lingua.local` параметры почты `LINGUA_SMTP_ADDR` (хост:порт), `LINGUA_SMTP_FROM`, `LINGUA_SMTP_USER` и `LINGUA_SMTP_PASSWORD` либо `LINGUA_SMTP_PASSWORD_FILE`. Используется STARTTLS. Для SMTP с TLS при подключении задайте `LINGUA_SMTP_TLS=implicit`. Почта используется для восстановления пароля и подтверждений записи, переноса и отмены. Для внешнего прокси задайте также `LINGUA_PUBLIC_URL` с HTTPS-адресом.
3. Для разовой оплаты через ЮKassa задайте `LINGUA_YOOKASSA_SHOP_ID` и `LINGUA_YOOKASSA_SECRET` либо `LINGUA_YOOKASSA_SECRET_FILE`. В `school.json` должны быть настоящие цены обоих пакетов и ссылки на контакты, условия и политику конфиденциальности. В кабинете магазина включите уведомления `payment.succeeded` и `payment.canceled` на `https://ВАШ_ДОМЕН/api/payment-notifications`. Пакет активируется после проверки платежа у провайдера. Чеки, если нужны для вашего магазина, включите в настройках выбранного решения до приёма платежей. Запросы на возврат принимаются через поддержку и обрабатываются оператором в кабинете провайдера.

Файлы `.env.lingua.local`, `compose.override.yaml` и `school.json` исключены из Git. Файлы с секретами задавайте путями внутри контейнера, монтируя их только для чтения. В режиме Docker с HTTPS команды обслуживания должны использовать тот же набор Compose-файлов и проект.

### Резервные копии

```sh
./data.sh backup
./data.sh restore backups/ИМЯ-ФАЙЛА.json
```

Для HTTPS-конфигурации добавляйте префикс `LINGUA_DEPLOY_HTTPS=1`. Команда восстановления останавливает приложение, проверяет копию и сохраняет прежние данные в томе перед заменой. Храните копии вне сервера с приложением. Файлы `school.json`, `schedule.json` и настройки среды сохраняются отдельно.

## Настройки сервера

| Переменная | По умолчанию | Назначение |
| --- | --- | --- |
| `LINGUA_ADDR` | `127.0.0.1:8080` | Адрес HTTP-сервера |
| `LINGUA_DATA` | `data/lingua.json` | Файл профилей, записей, абонементов и сессий |
| `LINGUA_WEB` | `web/dist` | Папка собранного React |
| `LINGUA_SCHEDULE` | Демо-расписание | Путь к исходному расписанию в JSON |
| `LINGUA_SCHOOL` | Примеры модулей | Путь к JSON с учебниками, профилем преподавателя и правилами школы |
| `LINGUA_SECURE_COOKIES` | `0` | Установите `1` при доступе через HTTPS |

Хранилище рассчитано на один экземпляр Go-сервера. Обновления сохраняются атомарной заменой файла. Сессия живёт 14 дней, пароли хранятся как хеши с солью.

### Расписание преподавателя

Передайте путь к JSON-файлу в `LINGUA_SCHEDULE` и перезапустите сервер. Слоты из этого файла отображаются в кабинете для справки; занятиями, добавленными через кабинет, можно управлять там же. Чтобы привязать групповой урок к модулю, добавьте `moduleId` из `school.json`. У каждого занятия должен быть уникальный стабильный `id`. При смене времени создайте новый слот. Уже созданные записи содержат время и ссылку на встречу, которые действовали на момент записи.

```json
[
  {
    "id": "chinese-group-2026-12-10-1900",
    "language": "chinese",
    "format": "group",
    "startsAt": "2026-12-10T19:00:00+03:00",
    "duration": 55,
    "capacity": 5,
    "meetingUrl": "https://example.org/replace-with-your-meeting"
  }
]
```

Языки — `english` и `chinese`. Форматы — `trial`, `individual` и `group`. Для первых двух вместимость равна 1, для мини-группы — от 2 до 5. Ссылка HTTPS доступна в кабинете записавшегося ученика. В режиме демо можно проверить запись, подключение к видео появляется с реальным расписанием.

## Изображение главной страницы

Изображение `web/public/images/lesson.webp` создано встроенным ImageGen. Запрос для генерации:

> Editorial lifestyle photograph for the right-hand hero of a contemporary language learning website, wide landscape 4:3 crop. A friendly young adult East Asian woman with short dark bob and warm smile wearing a light ivory knit sweater and subtle ivory wireless over-ear headphones sits at a honey oak desk, engaged in a video language lesson, looking at an open dark silver laptop near the lower left. Camera perspective at 3/4 from front, her face clearly visible in the central upper-right. A notebook, hand with pen, and an understated ceramic cup on the desk. Warm sidelight from the right, olive sage plaster wall behind, soft leaf shadows, a plant in right background. Natural skin texture, relaxed genuine moment, art-directed magazine photography, cinematic warmth, realistic proportions, muted greens and earthy honey tones. Frame from the waist up with room for small web overlays at lower right and top left. Fill the whole frame. No letters, no text, no graphic design, no interfaces, no logos or watermark.
