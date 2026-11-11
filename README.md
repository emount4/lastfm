# Last.fm для Telegram

Go-бот для одной группы. Участники отправляют ник Last.fm в личку боту, а привязки хранятся в Turso. В группе команды пишутся без слеша и работают в любом регистре.

| Команда | Что показывает |
| --- | --- |
| `info` | Справку и ссылку на личку бота |
| `playing` | Коллаж из обложек треков с числом слушателей в подписи |
| `topart [период]` | Топ 25 артистов группы. Примеры периодов — `10d`, `2w`, `3m`, `2y`. Без периода — за всё время. |
| `toptrk [период]` | Топ 25 треков группы, например `toptrk 2w` |
| `topalb [период]` | Топ 25 альбомов группы, например `topalb 3m` |
| `topusers [период]` | Топ 25 слушателей по числу скробблов, например `topusers 7d`. Без периода — за всё время. |
| `wka Radiohead` | Скробблы артиста по участникам |
| `wktrk Radiohead - Creep` | Скробблы трека по участникам |
| `wkalb Radiohead - OK Computer` | Скробблы альбома по участникам |

У трека и альбома можно указать только название, например `wktrk Creep`. Если совпадений несколько, выбери нужное кнопкой. В результатах `wk*` жирным выделено число скробблов участника. Под списком указана общая сумма.

В названиях для `wk*` можно писать `е` вместо `ё`. Если в запросе явно указана `ё`, бот ищет именно её.

Отправь `unlink` в личку боту, чтобы удалить привязку и убрать свой профиль из следующих расчётов. Чтобы вернуться, снова отправь ник Last.fm.

## Настройка

1. Скопируй [`.env.example`](.env.example) в `.env` и заполни токен бота, секрет вебхука, ID группы, API-ключ Last.fm, адрес и токен базы Turso libSQL.
2. В BotFather включи **Allow Groups** и выключи **Group Privacy**. Добавь бота обычным участником в группу, где ему разрешено отправлять сообщения. Если бот уже был в группе при смене **Group Privacy**, удали его из группы и добавь заново. [Telegram Privacy Mode](https://core.telegram.org/bots/features#privacy-mode)
3. Для Vercel импортируй репозиторий и добавь те же переменные в **Production**, кроме `PORT`. Проверь `GET /healthz` на публичном домене.
4. Зарегистрируй вебхук по адресу `https://<домен>/telegram/webhook` через `setWebhook`. Передай `TELEGRAM_WEBHOOK_SECRET` как `secret_token` и укажи `allowed_updates` как `["message","callback_query"]`. [Telegram setWebhook](https://core.telegram.org/bots/api#setwebhook)

## Локальный запуск

Нужен Go 1.25+. В PowerShell из корня проекта:

```powershell
$settings = Get-Content -Raw -LiteralPath .env | ConvertFrom-StringData
foreach ($entry in $settings.GetEnumerator()) {
    [Environment]::SetEnvironmentVariable($entry.Key, $entry.Value, 'Process')
}
go run ./cmd/server
```

Таблицы создаются при запуске. Проверки проекта — `go test ./...` и `go vet ./...`.
