# Progress

## Faz 0 tamamlandı (2026-10-13)

Repo iskeleti kuruldu; uygulama kodu yok.

### Araç sürümleri

| Araç | Sürüm |
|---|---|
| Go | 1.27.2 (linux/amd64) |
| Node / npm | 24.21.0 (LTS, nvm) / 11.19.0 |
| protoc | libprotoc 3.21.12 |
| protoc-gen-go | v1.36.12 |
| Docker / Compose | 29.8.1 / v5.5.1 |
| gh | 2.102.0 |
| GNU Make | 4.4.1 |

Bağımlılıklar: Go `google.golang.org/protobuf` v1.36.12, `github.com/coder/websocket` v1.8.15
(henüz import edilmiyor; `go mod tidy` Faz 1'den önce çalıştırılırsa go.mod'dan düşer).
Web: Svelte 5.57, Vite 8.3, TypeScript 6.0, `dompurify` 3.4.17, `vitest` 5.0.
`@types/dompurify` eklenmedi: DOMPurify 3.x kendi tiplerini içeriyor, `@types` paketi deprecated stub.

### Mumble proto

- Kaynak: `mumble-voip/mumble` etiketi **v1.5.939** (en güncel kararlı 1.5.x), `src/Mumble.proto` ve `src/MumbleUDP.proto`.
- Yerel değişiklik: başa kaynak yorumu ve `option go_package` eklendi
  (`gateway/proto/mumbleproto`, `gateway/proto/mumbleudp`). Yeniden üretim: `make proto`.
- Not: `mumblevoip/mumble-server:latest` imajı şu an sunucu sürümü **1.5.915** çalıştırıyor.

### Doğrulama

- `go build ./...` ve `go vet ./...` temiz.
- `npm run check`: 0 hata / 0 uyarı; `npm run build` çıktısı `gateway/web/dist`.
- `make gateway` → `bin/mumweb`, `bin/mumprobe`; binary embed edilmiş SPA'yı sunuyor.
  `dist` boşken "build yok" yer tutucu sayfası döndüğü ayrıca test edildi.
- `make test`: go test (test yok) + vitest (`--passWithNoTests`) geçti.
- `make dev`: iki container ayağa kalktı; gateway healthcheck `healthy`,
  `curl localhost:8080/healthz` → 200, `/` "mumweb" başlıklı sayfa ve JS asset'i 200,
  gateway container'ı root olmayan `mumweb` (uid 100) kullanıcısıyla çalışıyor,
  mumble-server TCP portu erişilebilir ve superuser şifresi ayarlandı.
- Yerelde 64738 başka bir Mumble sunucusu tarafından kullanıldığı için doğrulama
  `MUMBLE_PORT=64739` ile yapıldı (compose'ta host portu `MUMBLE_PORT` ile ayarlanabilir).

## Faz 1 — Mumble TCP istemci kütüphanesi + mumprobe (2026-10-13)

`gateway/internal/mumble`:
- `messages.go`: tip numaraları (Mumble.proto sırası, 0–26), `[uint16 BE tip][uint32 BE uzunluk]` framing,
  sunucuyla aynı boyut sınırı (0x7fffff). Bilinmeyen tipler debug loglanıp atlanıyor.
- `conn.go`: TLS (varsayılan doğrulama kapalı; `TLSVerify` ve SHA-256 parmak izi sabitleme destekli),
  `Version` (v1 + v2 = 1.5.0, release/os), `Authenticate` (opus=true), `ServerSync`'e kadar senkron handshake,
  `Reject` → `*RejectError`. Sonrasında okuma döngüsü + 15 sn ping, 60 sn okuma zaman aşımı,
  `context` ile iptal (Close tüm goroutine'leri bekliyor). Olaylar tamponlu Go kanalı ile yayınlanıyor.
- `state.go`: kanal/kullanıcı haritaları, kısmi `ChannelState`/`UserState` birleştirme (links_add/remove dahil),
  `user_id` alanı varsa kayıtlı kullanıcı.
- `audio.go`: `MumbleUDP.Audio` ↔ tünel yükü (`0x00` başlığı); legacy biçim `ErrLegacyAudio` ile atlanıyor.
- `events.go`: olay tipleri.

`cmd/mumprobe`: kanal ağacı + kullanıcıları yazdırır, olayları loglar (ses: ilk paket, her 50. paket ve terminator).
Bayraklar: `-addr -user -password -cert -key -verify -fingerprint -say -join -tx <süre> -exit -debug`.
`-tx` 20 ms'lik geçerli Opus sessizlik paketleri (`f8 ff fe`) gönderir; masaüstü istemcisi olmadan ses yolunu sınamak için.

### Doğrulama (dev sunucu 1.5.915, `localhost:64739`)
- `go vet`, `go test ./...` temiz: framing encode/decode ve hata durumları, kısmi UserState/ChannelState birleştirme,
  ses encode/decode (target=0 oneof'un kablo üzerinde bulunması, legacy tespiti).
- İki mumprobe: `listener` ve `talker -say 'merhaba <b>dünya</b>' -tx 1s`. Listener; talker'ın girişini,
  metni ve **protobuf formatlı** ses paketlerini (frame 0 … 49, son pakette `term=true`) TCP tüneli üzerinden aldı;
  legacy paket görülmedi. Talker çıkınca `userRemove` geldi.
- Yanlış SuperUser şifresi → `rejected (WrongUserPW)`; doğru şifreyle SuperUser `[registered]` görünüyor.
- **Bekleyen manuel doğrulama:** masaüstü Mumble istemcisiyle kanal listesi, masaüstünden konuşma ve `-say`
  mesajının masaüstünde görünmesi (kullanıcı tarafından yapılacak).
