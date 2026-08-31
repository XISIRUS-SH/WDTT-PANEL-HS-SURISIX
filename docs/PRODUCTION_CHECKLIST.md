# Production checklist

Перед публикацией релиза:

- [ ] CI: `gofmt`, `go test ./...`, `go vet ./...`, `go build ./...`.
- [ ] Проверить реальный `wdtt.service` и путь к бинарнику.
- [ ] Проверить backup/rollback installer на тестовом VPS.
- [ ] Проверить, что `/panel/api/health` отвечает без авторизации, а остальные API требуют session.
- [ ] Проверить logout с корректным CSRF и отклонение запроса без CSRF.
- [ ] Проверить 401 после истечения session.
- [ ] Проверить полный CRUD клиента через реальные admin-команды.
- [ ] Проверить export/import на тестовой БД.
- [ ] Проверить routing diagnostics перед включением proxy/WARP/WG режимов.
- [ ] Проверить live metrics при реальном трафике.
- [ ] Проверить 500 клиентов на staging без деградации admin lock.
- [ ] Проверить backup `passwords.json` перед массовыми изменениями.
- [ ] Не публиковать `passwords.json`, VK hashes, tokens, WireGuard private keys или SSH credentials.
- [ ] Для внешнего доступа предпочтительно reverse proxy + HTTPS; не выставлять panel HTTP в интернет без защиты сети.
