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

## Faz 2 — WebSocket köprüsü (2026-10-13)

- `internal/certs`: RSA 2048 kendinden imzalı istemci sertifikası (CN = kullanıcı adı, 20 yıl, PKCS#8 anahtar),
  `Load` PEM çiftini doğrular. Diskte hiçbir şey saklanmaz.
- `internal/bridge`: `GET /ws`. WS oturumu başına bir `mumble.Client`. Bölüm 6 protokolü.
  - Akış: `hello` (10 sn içinde) → sertifika yoksa üret + `cert` → Mumble'a bağlan →
    `channel`*, `user`* → `ready`. (Plan sırasından farkı: `ready` başlangıç durumundan **sonra** gelir;
    tarayıcı `ready` aldığında ağaç tamdır.)
  - `user`/`channel` her zaman birleştirilmiş tam kayıt olarak gönderilir; kök kanalın `parent`'ı `null`.
  - Eklenen alanlar: `ready.allowHtml`, `ready.messageLength`; `denied.type`; `text.private` (doğrudan mesaj);
    ağaç (tree) mesajları `channelIds`'e eklenir.
  - Hata/kapanış: `reject` (WS 1000), Mumble'a ulaşılamazsa/bağlantı koparsa `error` (WS 1013),
    protokol ihlali 1008. WS kapanınca Mumble bağlantısı kapanır ve tersi; `Shutdown()` tüm oturumları bitirir.
  - Sınırlar: Origin listesi (boşsa kütüphanenin aynı-host kontrolü), `MUMWEB_MAX_SESSIONS`,
    IP başına dakikada `MUMWEB_CONN_RATE` (10) yeni bağlantı, JSON 16 KB, ses çerçevesi 4 KB,
    oturum başına saniyede 100 ses paketi (fazlası düşürülür).
  - Loglarda şifre, anahtar ve sohbet içeriği yok.
- `cmd/mumweb`: Bölüm 10 ortam değişkenleri. Plana ek: `MUMWEB_CONN_RATE` (varsayılan 10) ve
  `MUMWEB_TRUST_PROXY` (varsayılan false; true ise istemci IP'si `X-Forwarded-For`'un en sağındaki değer —
  Caddy arkasında açılmalı, yoksa IP sınırı herkesi tek IP sayar).
- `mumble.Dial` artık `ServerSync`'ten sonra gelen `ServerConfig`'i de bekliyor (önceden `ready` içinde eksik kalıyordu).

### Önemli bulgu: Mumble autoban
Tüm web kullanıcıları Mumble'a gateway'in tek IP'sinden bağlanır. Mumble varsayılanı (120 sn'de 10 bağlantı →
300 sn global ban, **başarılı bağlantılar dahil**) testlerde gateway'i banladı (`Ignoring connection ... (Global ban)`).
Dev compose'a `MUMBLE_CONFIG_AUTOBAN_SUCCESSFUL_CONNECTIONS=false` eklendi. Production compose için karar Faz 7'de
(bkz. "Açık konular").

### Doğrulama
- Birim: ikili çerçeve encode/decode, kök kanal `parent:null`, token bucket, IP sınırı, Origin reddi (403),
  oturum sınırı (503), `hello` olmadan mesaj → 1008.
- Entegrasyon (`MUMWEB_TEST_MUMBLE_ADDR=localhost:64739 go test ./internal/bridge`, 5 tekrar): Go WS istemcisi
  `hello` → `cert`, `channel`, `user`, `ready`; ping/pong; metin iki yönde; `setSelf` gözlemcide görünüyor;
  ses iki yönde (frame no + terminator korunuyor); WS kapanınca gözlemci `UserRemove` alıyor;
  aynı sertifikayla ikinci girişte sertifika hash'i aynı; yanlış SuperUser şifresi → `reject WrongUserPW`;
  tüm oturumlar bitince goroutine sayısı başlangıca dönüyor.
- Docker dev stack'teki gerçek gateway'e Node (yerleşik WebSocket) istemcisi bağlandı; mumprobe gözlemci kullanıcıyı,
  metni ve çıkışı gördü.
- **Bekleyen manuel doğrulama:** WS kapanınca kullanıcının masaüstü istemcide düşmesi.

## Faz 3 — Arayüz iskeleti (2026-10-13)

- `web/src/lib/ws.ts`: `ReconnectingSocket` — 1, 2, 4, 8 … 30 sn backoff (`ready` sonrası sıfırlanır),
  `online` olayında bekleme atlanır, 20 sn'de bir `ping`; 10 sn `pong` gelmezse bağlantı ölü sayılıp yeniden kurulur.
  1008 (protokol ihlali) sonrası yeniden denenmez.
- `lib/client.ts`: eylemler (connect/disconnect/joinChannel/setSelf/sendText) ve sunucu mesajlarının işlenmesi.
  `reject`, kick/ban (`userRemove` kendi oturumumuz) ve geçersiz sertifika → yeniden bağlanma durur, giriş ekranında
  Türkçe hata. Diğer kopmalarda yeniden bağlanılır; sohbet geçmişi korunur, sunucu durumu her bağlantıda sıfırlanır.
- `lib/stores/session.svelte.ts` (Svelte 5 runes, `SvelteMap`), `lib/certStore.ts` (IndexedDB),
  `lib/settings.ts` (localStorage, takma ad), `lib/sanitize.ts` (DOMPurify: `b i u s strong em a br p img span`;
  link yalnız http/https/mailto ve yeni sekmede; img yalnız `data:image/*;base64`). Giden metin HTML-escape edilir.
- Bileşenler: `Login`, `ChannelTree` (özyinelemeli; sıralama position → ad; çift tık/Enter ile kanala geç),
  `Chat` (Enter gönderir, Shift+Enter yeni satır; kanala giriş/çıkış satırları), `Controls` (mute/deaf/çıkış),
  `StatusBar`. ≤640 px'de Kanallar/Sohbet sekmeleri. Koyu tema varsayılan, açık tema için `data-theme="light"` hazır.
- Gateway: `error` mesajlarına `code` alanı (`invalid_certificate`, `mumble_unavailable`, `disconnected`);
  tarayıcıdan 60 sn hiçbir şey gelmezse oturum kapanır.
- mumprobe: `-mkchannel` (test kanalı oluşturmak için) ve kullanıcı logunda sertifika hash'i.

### Doğrulama
- `npm run check` 0 hata; Vitest: ikili çerçeve (Go testiyle aynı baytlar), backoff, `textToHtml`.
- Docker dev stack + geçici headless Chromium (browserless imajı, dev ağına bağlı) + puppeteer-core betiği
  (scratchpad'de, repoya eklenmedi), mumprobe gözlemci ile:
  giriş → kullanıcı Mumble'da görünüyor, ağaçta gözlemci görünüyor; sohbet iki yönde; `<script>`, `onerror`'lu img ve
  `javascript:` link etkisiz; giriş/çıkış satırları; çift tıkla iki kez kanal değişimi Mumble'a yansıyor;
  mute/deaf Mumble'a yansıyor ve ikonlar görünüyor; sayfa yenilenince takma ad hatırlanıyor ve **sertifika hash'i aynı**;
  çıkış kullanıcıyı düşürüyor; yanlış SuperUser şifresi → "Bu takma ad kayıtlı; şifre veya sertifika yanlış.";
  390 px mobil düzen.
- Gateway container'ı yeniden başlatıldı → arayüz "Yeniden bağlanılıyor (1. deneme, 1 sn)" gösterip kendiliğinden
  bağlandı, sohbet geçmişi korundu.
- Hayalet oturum: aynı ad + aynı sertifikayla ikinci bağlantı `ready` alıyor, Mumble eski oturumu düşürüyor
  (Wi-Fi kopması sonrası yeniden bağlanma `UsernameInUse`'a takılmaz).
- **Bekleyen manuel doğrulama:** masaüstü Mumble'da tarayıcı kullanıcısının görünmesi, iki yönlü kanal değişimi ve sohbet,
  masaüstünden taşınınca tarayıcıda güncellenmesi, kayıtlı kullanıcı olarak kalma (kendini kaydetme masaüstünden/yönetici).

## Açık konular
- Production'da Mumble autoban ayarı (Faz 7): en az `autobanSuccessfulConnections=false`. Başarısız girişler
  (yanlış şifre) hâlâ gateway IP'si adına sayılır; 120 sn'de 10 yanlış şifre tüm web kullanıcılarını 5 dk keser.
