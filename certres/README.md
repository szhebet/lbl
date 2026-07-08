# Сертификаты для TWA Android-приложения

Этот каталог содержит скрипты и файлы сертификатов, необходимые для сборки
Trusted Web Activity (TWA) APK для Android.

## Содержимое

| Файл | Назначение |
|------|------------|
| `ca.crt` | Корневой CA-сертификат (запаковывается в APK как raw-ресурс) |
| `ca.key` | Закрытый ключ CA (не распространять) |
| `server.crt` | Сертификат сервера для TLS (используется в Go) |
| `server.key` | Закрытый ключ сервера для TLS |
| `server.p12` | PKCS12-контейнер сертификата сервера |
| `android.keystore` | Keystore для подписи APK |
| `assetlinks.json` | Digital Asset Links для TWA-верификации |

## Использование

### 1. Генерация сертификатов

```bash
cd certres
chmod +x *.sh

# Шаг 1: CA + серверные сертификаты
./generate-certs.sh

# Шаг 2: Keystore для подписи APK
./generate-keystore.sh

# Шаг 3: Digital Asset Links (содержит SHA256 отпечаток из keystore)
./generate-assetlinks.sh
```

### 2. Развёртывание assetlinks.json

Файл `assetlinks.json` должен быть доступен по URL `/.well-known/assetlinks.json`
на том же домене, который открывается в TWA.

Для интеграции с Go-приложением добавьте в `main.go`:

```go
r.GET("/.well-known/assetlinks.json", func(c *gin.Context) {
    c.File("./certres/assetlinks.json")
})
```

### 3. Подключение TLS в Go-приложении

```go
go func() {
    log.Fatal(http.ListenAndServeTLS(":443", "certres/server.crt", "certres/server.key", r))
}()
```

### 4. Сборка APK

APK собирается в Docker-контейнере вместе с основным приложением.
См. `Dockerfile` — стадия `android-builder`.

### 5. Установка CA на Android

Для работы с самоподписанным сертификатом:
1. Скиньте `ca.crt` на телефон
2. Настройки → Безопасность → Доверенные сертификаты → Установить
3. Или APK доверяет CA через `network_security_config.xml` (встроено)

## Важно

- Сертификаты, сгенерированные этими скриптами, предназначены для
  разработки и домашнего использования.
- Для публичного доступа используйте Let's Encrypt или другой доверенный CA.
- Keystore пароль по умолчанию: `android` — смените для production.
- Файлы `*.key` и `android.keystore` добавлены в `.gitignore`.
