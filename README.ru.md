# ITG Ray

[English](README.md)

ITG Ray — десктопный VLESS VPN-клиент для Linux и Windows на базе
[sing-box](https://github.com/SagerNet/sing-box) и
[Xray-core](https://github.com/XTLS/Xray-core), с интерфейсом на Electron и
привилегированным helper-демоном, благодаря которому сам GUI никогда не
запускается от root.

![Главное окно](docs/screenshots/main.png)

## Возможности

- **VLESS + подписки** — добавляй серверы по ссылкам `vless://` или через
  URL подписки, с автообновлением.
- **Режим TUN** — туннелирование всей системы через виртуальный интерфейс с
  FakeIP DNS; локальные inbound'ы SOCKS (`127.0.0.1:1080`) и HTTP
  (`127.0.0.1:8888`) остаются доступны.
- **Режим системного прокси** — более лёгкая альтернатива, которая просто
  выставляет прокси на уровне ОС.
- **Правила маршрутизации** — редактор правил с drag-and-drop (домены, IP,
  GeoIP/Geosite) и действиями proxy/direct/block для каждого правила.
- **Наблюдаемость** — логи ядра в реальном времени, статистика трафика и
  замер задержки.
- **Двуязычный интерфейс** — английский и русский.

## Установка

### Linux

- **Arch Linux (AUR):** `yay -S itgray-bin`, затем
  `sudo systemctl enable --now itgray-helper.service`
- **AppImage:** возьми `ITGRay-<version>.AppImage` со страницы
  [Releases](https://github.com/IvanTopGaming/ITG_Ray/releases), сделай его
  исполняемым и запусти. Для режима TUN требуется, чтобы встроенный helper
  работал как служба systemd — для TUN рекомендуется установка через
  AUR/тарболл.

### Windows

Скачай и запусти `ITGRay-Setup-<version>.exe` со страницы
[Releases](https://github.com/IvanTopGaming/ITG_Ray/releases). Установщик
регистрирует helper-службу и устанавливает драйвер Wintun.

## Сборка из исходников

Требуется: Go 1.23+, Node 22+, npm. Для кросс-сборки установщика Windows дополнительно нужен wine.

```bash
git clone https://github.com/IvanTopGaming/ITG_Ray
cd ITG_Ray
(cd cmd/itgray-electron && npm ci && cd frontend && npm ci)
bash scripts/build-linux.sh     # AppImage + binaries in dist/
bash scripts/build-windows.sh   # NSIS installer (cross-compiled from Linux)
```

## Архитектура

```
Electron GUI ──IPC──▶ bridge ──HTTP/unix──▶ itgray-helper (root, systemd/service)
                                                  │
                                          spawns sing-box / xray
```

Helper владеет всем привилегированным (интерфейс TUN, маршруты, DNS); GUI
общается с ним через локальный API и может перезапускаться независимо —
активный туннель переживает перезапуск GUI.

## Лицензия

GPL-3.0 — см. [LICENSE](LICENSE). Список используемых сторонних компонентов
находится в [docs/THIRD_PARTY.md](docs/THIRD_PARTY.md).
