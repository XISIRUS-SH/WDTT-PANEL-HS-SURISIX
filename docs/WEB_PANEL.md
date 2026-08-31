# WDTT Web Panel

Панель встроена в основной `wdtt-server`. Отдельный процесс и вторая admin-БД не создаются.

## Запуск

По умолчанию `127.0.0.1:8787/panel/`.

```text
--panel-listen 0.0.0.0:8787
```

или:

```text
WDTT_PANEL_LISTEN=0.0.0.0:8787
```

`-` отключает панель.

## Авторизация

Вход использует `db.MainPassword`. Пароль не сохраняется в браузере. После входа выдаются
HttpOnly session cookie и отдельный CSRF token.

## Admin-контур

Все admin-команды проходят через `dbMutex` и реальный `executeAdminCommand(..., wgDev, true)`.
Это сохраняет существующие live effects: WRAP keys, WireGuard peers, DNS/default ports,
public IP и cleanup.

`admin.sock` уже использует тот же `dbMutex`, поэтому Web и Unix admin-входы сериализуются.
Другие прямые пути к `db` должны также продолжать использовать `dbMutex`.

## CAPTCHA

Автоматическое решение CAPTCHA не реализуется. CAPTCHA проходит только пользователь.
