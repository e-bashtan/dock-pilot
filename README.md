# Barn (Амбар)

Панель для управления сайтами и ботами на Docker на VPS: деплой, nginx, SSL, мониторинг, Telegram-уведомления.

Стек: Go API · Next.js UI · PostgreSQL · Docker · nginx · certbot.

> **Ребрендинг:** DockPilot → Barn. Старые установки работают, новые используют `/opt/barn` и barn-* образы.

---

## VPS: первая установка

**Нужно:** Ubuntu/Debian, Docker (скрипт может поставить сам), nginx.

**С доменом:** A-запись на сервер, порты 80 и 443, `--email` для Let's Encrypt.

**Без домена:** панель по `http://IP:8888` (порт настраивается через `PANEL_HTTP_PORT`), SSL для панели не нужен.

Скачайте скрипт в файл (не `curl | bash` — иначе может оборваться на миграциях):


```bash
curl -fsSL -H "Accept: application/vnd.github.raw+json" \
  "https://api.github.com/repos/ebasht/barn/contents/scripts/install.sh?ref=main" \
  -o /tmp/barn-install.sh

# С доменом и HTTPS:
sudo bash /tmp/barn-install.sh \
  --domain panel.example.com \
  --email you@example.com \
  --version latest

# Без домена (доступ по IP:порт):
sudo bash /tmp/barn-install.sh --version latest
```

Опции:

| Флаг | Зачем |
|------|--------|
| `--domain` / `--email` | Домен панели и email для Let's Encrypt |
| `--version v0.1.19` | Конкретный релиз вместо `latest` |
| `--skip-cert` | С `--domain`: HTTP без TLS для панели |
| `--skip-packages` | Docker/nginx/certbot уже установлены |
| `--reset-db` | Сбросить volume PostgreSQL (данные панели пропадут) |
| `--install-dir` | Путь установки (по умолчанию `/opt/barn`, для legacy `/opt/dock-pilot`) |

После установки:

- С доменом: `https://panel.example.com`
- Без домена: `http://VPS_IP:8888` (откройте порт в firewall)
- API-токен: в выводе скрипта и в `/opt/barn/credentials.txt`
- Файлы: `/opt/barn` (новые) или `/opt/dock-pilot` (legacy)

В UI введите токен (хранится в `localStorage` до выхода). На телефоне можно войти по QR с десктопа.

---

## VPS: обновление

Повторный `install.sh` **не** подтягивает новые образы. Для обновления:

```bash
curl -fsSL -H "Accept: application/vnd.github.raw+json" \
  "https://api.github.com/repos/ebasht/barn/contents/scripts/barn-upgrade.sh?ref=main" \
  -o /tmp/barn-upgrade.sh

sudo bash /tmp/barn-upgrade.sh v0.1.7
```

или последний релиз:

```bash
sudo bash /tmp/barn-upgrade.sh latest
```

**Legacy (dock-pilot):** скрипт `dock-pilot-upgrade.sh` работает как wrapper к `barn-upgrade.sh`.

**Добавить домен и SSL для панели** (если ставили без `--domain`):

```bash
sudo bash /tmp/barn-upgrade.sh latest \
  --domain panel.example.com \
  --email you@example.com
```

Скрипт: скачивает release (barn-*.tar.gz или dock-pilot-*.tar.gz) → `docker load` → миграции → пересоздаёт `postgres`, `api`, `frontend` → при `--domain` настраивает nginx и Let's Encrypt. Токен и данные БД сохраняются.

Проверка: версия в шапке панели (например `v0.1.19`).

Релизы: [github.com/ebasht/barn/releases](https://github.com/ebasht/barn/releases) (dual: barn-*.tar.gz + dock-pilot-*.tar.gz)

---

## Локальная разработка

Требования: Docker, Go 1.22+, Node.js 20+.

```bash
make setup    # зависимости + .env
make up       # PostgreSQL + миграции
make dev-run  # API :8080 + UI :3000
```

`DEPLOY_MODE=stub` в `.env` — деплой без реального Docker (только логи). На VPS — `DEPLOY_MODE=real`.

Полезное: `make migrate`, `make down`, `make reset`, `make docker-export` (образы для VPS).

---

## Авторизация

- Сервер: `API_TOKEN` в `.env` (≥ 16 символов)
- Клиент: `Authorization: Bearer <token>` на все `/api/*`
- `GET /health` — без токена
- SSE-логи: `?token=...` в URL

---

## Сборка релиза (maintainer)

```bash
make release VERSION=v0.1.0
git tag v0.1.0 && git push origin v0.1.0
```
