# Derleme, test ve yayınlama

[English](../BUILDING.md) | **Türkçe**

Bu sayfaya yalnızca mumweb'i kendiniz derlemek, kodu değiştirmek, testleri çalıştırmak veya sürüm yayınlamak için
ihtiyacınız var. Sadece çalıştırmak için [README](../../README.tr.md#kurulum-yeni-bir-sunucuda) içindeki hazır Docker
imajını kullanın.

## Kaynaktan derleme ve çalıştırma

**Yerel HTTPS test ortamı** (Caddy + Mumble + mumweb, mkcert sertifikası, `https://mumweb.test`; Windows için adım
adım anlatım): [DEVELOPMENT.md](DEVELOPMENT.md) — `deploy/local/` içinde `docker compose up -d --build`.

Aşağıdakiler kaynak koddan geliştirme içindir. Gereksinimler: Go 1.27+, Node 24 LTS, Docker + Compose, GNU Make; `protoc` + `protoc-gen-go` yalnızca `make proto` için.

```sh
cp deploy/.env.example deploy/.env   # MUMBLE_SUPERUSER_PASSWORD; port doluysa MUMBLE_PORT
make dev            # dev Mumble sunucusu + gateway (http://localhost:8080)
cd web && npm run dev   # arayüz için Vite (http://localhost:5173, /ws gateway'e aktarılır)
make gateway        # bin/mumweb ve bin/mumprobe (arayüz gömülü)
make test           # Go testleri + Vitest
make docker         # production imajı (mumweb:dev)
make proto          # gateway/proto'dan Go kodu üret
```

Mikrofon ve WebCodecs güvenli bağlam gerektirir: `localhost` sorunsuzdur; başka bir makineden test ederken HTTPS kullanın.

`bin/mumprobe` Mumble sunucusunu doğrudan yoklayan bir CLI'dır: kanal ağacını yazdırır, olayları ve ses paketlerini
loglar; `-say` metin gönderir, `-play` Opus paket dosyası çalar, `-tx` sessizlik gönderir, `-mkchannel` kanal oluşturur.

Gateway entegrasyon testleri gerçek bir Mumble sunucusu ister:
`MUMWEB_TEST_MUMBLE_ADDR=localhost:64738 go test ./internal/bridge` (dev stack'in portu).

| Yol | İçerik |
|---|---|
| `gateway/cmd/mumweb` | HTTP/WebSocket gateway (arayüzü sunar, `/ws`, `/healthz`) |
| `gateway/cmd/mumprobe` | Mumble yoklama CLI'ı |
| `gateway/internal/mumble` | Mumble TCP istemci kütüphanesi (TLS, framing, durum, ses tüneli) |
| `gateway/internal/bridge` | WebSocket oturumu ↔ Mumble istemcisi |
| `gateway/internal/certs` | İstemci sertifikası üretimi |
| `gateway/proto` | Resmî `Mumble.proto` / `MumbleUDP.proto` + üretilen Go kodu |
| `web/src/lib` | WS istemcisi, ses hattı (`audio/`), durum store'ları |
| `web/src/components` | Arayüz bileşenleri |
| `web/src/admin`, `web/admin` | Yönetim paneli: `/admin/` adresinde sunulan ikinci Vite sayfası (kendi istemcisi, ses yok) |
| `deploy/` | Production compose, Caddyfile, `.env.example`, dev compose |
| `deploy/local/` | Yerel HTTPS test ortamı (mkcert), bkz. [DEVELOPMENT.md](DEVELOPMENT.md) |

İlerleme ve doğrulama notları: [PROGRESS.md](PROGRESS.md). Yapılacaklar: [TODO.md](TODO.md).

## Testler

```sh
make test                                                    # Go birim testleri + Vitest
MUMWEB_TEST_MUMBLE_ADDR=localhost:64738 MUMWEB_TEST_SUPERUSER_PASSWORD=<şifre> go test ./internal/bridge   # gerçek sunucuya karşı (gateway/ içinde); SuperUser şifresi yönetim testlerini açar
```

- Go: güvenlik başlıkları, IP başına oturum sınırı, kontrol mesajı seli (entegrasyon), fuzz hedefleri (`go test ./internal/bridge -run '^$' -fuzz FuzzClientMessage -fuzztime 1m`; aynı şekilde `FuzzParseAudioFrame` ve `internal/mumble` içinde `FuzzReadFrame`, `FuzzDecodeAudio`, `FuzzStateMessages`); yönetim paneli (yasak listesi, kayıtlı kullanıcılar ve sahte sunucuyla ACL gidiş-dönüş; yönetici şifresi, kilit ayrımı, yasak/ACL doğrulama, hız yavaşlatma), framing encode/decode ve hatalar, kısmi `UserState`/`ChannelState` birleştirme, `MumbleUDP.Audio` encode/decode,
  sahte TLS sunucusuyla istemci (handshake, bilinmeyen mesaj tipleri, reject, zaman aşımı, parmak izi, kapanış),
  sertifika üretimi, köprü ikili çerçeveleri, Origin/oturum/IP sınırları; entegrasyon: WS ↔ Mumble metin, ses,
  durum, sertifika kalıcılığı, goroutine sızıntısı.
- Vitest: ikili ses çerçeveleri, jitter buffer, çerçeveleme/yeniden örnekleme, VAD, bitrate, PEM, yeniden bağlanma süresi.

Manuel kabul testleri: [TESTING.md](TESTING.md).

## Docker imajını yerelde derleme

```sh
make docker                       # makineniz için mumweb:dev derler
```

Production yığınını Docker Hub imajı yerine kendi derlemenizle çalıştırmak için override dosyasını ekleyin:

```sh
cd deploy
docker compose -f docker-compose.yml -f docker-compose.build.yml up -d --build
```

## Sürüm yayınlama (bakımcılar)

İmajlar Docker Hub'a [`aoztrk82/mumweb`](https://hub.docker.com/r/aoztrk82/mumweb) olarak `linux/amd64` ve
`linux/arm64` için gider. Dockerfile Go'yu derleme makinesinde çapraz derler, bu yüzden derleme için CPU emülasyonu
gerekmez.

```sh
docker login                                                                   # bir kez
docker buildx create --name mumweb-builder --driver docker-container --bootstrap   # bir kez
make publish VERSION=0.3.0       # :latest ve :0.3.0 gönderir
```

Sonra kontrol edin: `docker buildx imagetools inspect aoztrk82/mumweb:latest` iki platformu da listelemeli ve temiz bir
`docker run -p 8080:8080 aoztrk82/mumweb:0.3.0` için `curl localhost:8080/healthz` `ok` dönmeli. Docker Hub sayfası (kısa açıklama ve README) [DOCKERHUB.md](../DOCKERHUB.md) dosyasından gelir (yalnızca İngilizce).
Değiştirdikten sonra `make hub-readme` çalıştırın; betik Docker Desktop'ın mevcut Docker girişini (credential helper)
kullanır, ek token gerekmez. GitHub'daki README ile tutarlı tutun, doğru kaynak odur.

Sürümü
`gateway/cmd/mumweb/main.go` içindeki `release` ve `Makefile`'daki varsayılan `VERSION` ile birlikte artırın,
yayını [PROGRESS.md](PROGRESS.md)'ye yazın.
