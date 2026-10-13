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

## Faz 4 — Ses alma (2026-10-13)

- `web/src/lib/audio/context.ts`: tek `AudioContext` (48 kHz, `interactive`), giriş tıklamasında açılıyor.
- `audio/receiver.ts`: konuşmacı başına WebCodecs `AudioDecoder` (opus, 48 kHz, mono); geride kalan/yinelenen
  paket (`frameNumber` ≤ son) atılır; terminator'ün PCM'i gidince worklet'e `end`; 300 ms paket gelmezse
  "konuşmuyor"; 30 sn boşta kalan decoder kapatılır; decoder hatasında yeniden oluşturulur.
  Kazanç = yerel ses seviyesi (0–200 %, yerel susturma) × `volume_adjustment` (0 ise 1).
  `AudioDecoder` opus desteklemiyorsa üstte uyarı bandı (Faz 9).
- `audio/playback-worklet.ts` + `audio/jitter.ts`: kullanıcı başına jitter buffer (60 ms ön dolum, en fazla 200 ms,
  taşarsa en eski atılır, eksikse sessizlik + yeniden ön dolum, terminator'de kalan çalınıp akış biter),
  bağlam 48 kHz değilse doğrusal yeniden örnekleme, tüm kullanıcılar karıştırılıp [-1, 1]'e kırpılır.
- Sağırlaştırınca worklet anında temizlenir, paketler alınır ama çalınmaz (konuşma göstergesi çalışmaya devam eder).
- Arayüz: konuşan kullanıcının avatarında yeşil halka; başka kullanıcıya tıklayınca ses seviyesi kaydırıcısı,
  "Benim için sustur", "Sıfırla" (kullanıcı adına göre localStorage'da).
- mumprobe: `-play <dosya>` (uzunluk önekli Opus paketleri, TOC'tan süre hesaplanıp gerçek zamanlı gönderilir).

### Protokol düzeltmesi: `frame_number` birimi
Plan 5.3 "20 ms çerçeve başına +1" diyor. Resmî istemci (v1.5.939 `AudioInput.cpp`/`AudioOutputSpeech.cpp`):
`iFrameSize = sampleRate / 100` (10 ms), gönderilen `frameNumber = iFrameCounter - iBufferedFrames` 10 ms'lik
çerçeve sayar; alıcı jitter buffer'ı `timestamp = iFrameSize * frameNumber`, `span = paketteki örnek sayısı` kullanır.
Yani 20 ms'lik pakette sayaç **+2** artmalı (+1 olursa masaüstü alıcıda zaman damgaları çakışır).
mumprobe `-tx`/`-play` buna göre düzeltildi; Faz 5'te tarayıcı da `örnek sayısı / 480` kadar artıracak.

### Doğrulama
- Vitest: jitter buffer (ön dolum, kazanç/karıştırma, 200 ms sınırı, terminator sonrası kısa patlama, underrun,
  44.1 kHz yeniden örnekleme). `npm run check` temiz.
- Uçtan uca (headless Chromium, `--unsafely-treat-insecure-origin-as-secure=http://gateway:8080`): test sayfasında
  tarayıcının `AudioEncoder`'ı ile 440 Hz ve 660 Hz (genlik 0.3) Opus paketleri üretildi, `mumprobe -play` ile
  Mumble'a gönderildi; tarayıcı çıkışı AnalyserNode ile ölçüldü:
  - tek konuşmacı: RMS 0.212 (= 0.3/√2, kayıpsız seviye), 440 Hz -24 dB, diğer tüm bölmeler < -84 dB; konuşma göstergesi 1.
  - bitince RMS 0, gösterge 0.
  - iki konuşmacı aynı anda: 440 Hz -24 dB ve 660 Hz -25 dB, RMS 0.301 (√2 kat), iki gösterge birlikte.
  - Ton660 yerelde susturulunca 660 Hz -112 dB, 440 Hz korunuyor.
  - sağırlaştırılmışken RMS 0.
- **Bekleyen manuel doğrulama:** masaüstünden gerçek konuşmanın tarayıcıda temiz duyulması (ve 2–3 kişi aynı anda).

## Faz 5 — Ses gönderme (2026-10-13)

- `audio/capture-worklet.ts` + `audio/frames.ts`: mikrofon → 20 ms / 960 örnek 48 kHz mono çerçeve + RMS;
  bağlam 48 kHz değilse doğrusal yeniden örnekleme. Firefox'ta akış/bağlam örnekleme hızı uyuşmazsa ayrı bir
  yakalama bağlamı açılır.
- `audio/transmitter.ts`: `getUserMedia` (AEC, NS [ayarlanabilir], AGC, mono; kayıtlı cihaz yoksa varsayılana düşer),
  `AudioEncoder` (opus, 48 kHz, mono, 20 ms, complexity 9, DTX kapalı). Bitrate = min(40 000, maxBandwidth − 19 600):
  resmî `getNetworkBandwidth` TCP-modu ek yükü. Mute/deaf/sunucu mute/suppress iken gönderilmez (seviye göstergesi çalışır).
  - **Terminator:** konuşma bitince son çerçeve de kodlanır, `flush()` tamamlanana kadar çıktılar tutulur ve
    **en son paket** terminator olarak işaretlenir. (İlk sürümde flush'ın ürettiği lookahead kuyruğu terminator'den
    sonra ayrı bir paket olarak gidiyordu; testte yakalandı ve düzeltildi.)
  - Frame numarası paket başına +2 (10 ms birim, bkz. Faz 4 notu).
- PTT: ayarlanabilir tuş (varsayılan `V`; metin alanında yazarken tetiklenmez; pencere odağı kaybolunca bırakılır)
  ve kontrol çubuğunda basılı tutulan buton. VAD: anında açılış, 300 ms bekleme; eşik seviye çubuğunun üstündeki
  kaydırıcıyla (-60…0 dBFS ölçeği) ayarlanır. Ayarlar localStorage'da (`mumweb.audio`).
- Ayarlar penceresi (Faz 6'da genişleyecek): mikrofon seçimi, PTT tuşu atama. Mikrofon izni yoksa
  "yalnızca dinleyebilirsiniz" uyarısı, oturum devam eder.

### Doğrulama
- Vitest: FrameAssembler (48 kHz bölme, 44.1→48 kHz), VAD bekleme süresi, bitrate sınırı, seviye ölçeği.
- Uçtan uca: Chromium'a sahte mikrofon olarak WAV (1,5 sn 1 kHz + 1,5 sn sessizlik, döngü); A konuşur, B (mute) dinler
  ve çıkışı FFT ile ölçülür, mumprobe paketleri loglar:
  - VAD: konuşma blokları 0→180, 182→364 … (90 paket = 1,5 sn + 300 ms), sonda `term=true`;
    sessiz bölümde **paket yok**; terminator sonrası başıboş paket yok.
  - B'de 1 kHz duyuluyor (-16 dB; tarayıcı AGC'si +8 dB yükseltiyor). Tarayıcı ses işlemesi kapalıyken:
    RMS 0.212 (birebir) ve diğer frekanslar < -88 dB, yani hat temiz.
  - PTT: tuşsuz 3 sn → 0 paket; 2 sn basılı → 52 paket (506…712, +2 adım); terminator bırakıştan 22 ms sonra.
  - Sohbete "vvv" yazmak paket göndermiyor; VAD + mute → 0 paket.
  - Eşik 1.0 → ton varken 0 paket; 0.3 → paket gidiyor; ayar kalıcı.
- **Bekleyen manuel doğrulama:** tarayıcıdan konuşmanın masaüstünde net duyulması ve PTT bırakılınca masaüstündeki
  konuşma göstergesinin hemen sönmesi; gerçek mikrofonla yankı engelleme (Chrome'un AEC'si WebAudio çıkışını
  her platformda referans almayabilir; kulaklıksız kullanımda yankı olursa not edilmeli).

## Faz 6 — Cilalama (2026-10-13)

- Ayarlar ekranı: mikrofon, hoparlör/kulaklık (`AudioContext.setSinkId`; desteklemeyen tarayıcıda açıklama),
  gürültü engelleme (mikrofonu yeniden açar), konuşma modu, PTT tuşu atama, VAD eşiği (canlı seviye çubuğu ile),
  tema (koyu varsayılan / açık), giriş-çıkış sesleri, sertifika: dışa aktar (PEM: sertifika + anahtar),
  içe aktar (PEM), sıfırla (uyarı ile); değişiklik sonrası "Şimdi yeniden bağlan".
  Not: masaüstü Mumble'ın `.p12` dışa aktarımı desteklenmiyor (PEM'e çevrilmesi gerekir, ör. `openssl pkcs12 -nodes`).
- Mobil (≤ 640 px): sekmeli düzen + PTT modunda kontrol çubuğunun üstünde tam genişlikte 72 px bas-konuş butonu
  (dokunmatik, pointer capture, uzun basma menüsü kapalı); kontrol çubuğu ikonlara iner, yatay taşma yok.
  641–900 px'de buton etiketleri gizlenir, çubuk tek satır.
- Bildirim sesleri: kanalımıza biri girince yükselen, çıkınca alçalan kısa iki ton (WebAudio ile sentez,
  sağırken çalmaz, kapatılabilir).
- PWA: `manifest.webmanifest` (standalone, tr), ikonlar (192, 512, maskable 512, apple-touch 180; SVG'den üretildi),
  `sw.js` yalnızca statik dosyaları önbelleğe alır (gezinme: önce ağ; `/assets/*`: önce önbellek; `/ws`, `/healthz` hariç).
  Gateway: `.webmanifest` MIME tipi, `/assets/*` için 1 yıl `immutable`, diğerleri `no-cache`, `nosniff`.
- Bağlantı durumu çubuğu ve Türkçe hata/reject metinleri Faz 3'te yapılmıştı.

### Doğrulama
- Vitest: PEM paketi ayrıştırma (gidiş-dönüş, RSA anahtar, eksik parça).
- Uçtan uca (headless Chromium):
  - Chrome `Page.getInstallabilityErrors` → boş, manifest hatası yok, service worker aktif.
  - Biri kanala girip çıkınca 4 osilatör (2 ton × 2); "sesler" kapalıyken 0.
  - Tema açık → `data-theme="light"`, arka plan değişiyor.
  - Sertifika: dışa aktarılan PEM'de sertifika + anahtar var; sıfırla → yeni kimlik (hash değişti);
    içe aktar → yeniden bağlanınca **orijinal hash geri geldi**.
  - Android Chrome emülasyonu (390×800, dokunmatik): büyük PTT butonuna 1,2 sn dokunma → 66 paket + terminator.
  - 800 px ve 390 px'de kontrol çubuğu tek satır, sayfa yatay taşmıyor (PTT ve VAD modlarında).
- **Bekleyen manuel doğrulama:** gerçek Android Chrome'da "Ana ekrana ekle" → tam ekran açılış ve bas-konuş
  (HTTPS gerekir; Faz 7 dağıtımından sonra).

## Faz 7 — Dağıtım (2026-10-13)

- `Dockerfile` (Faz 0'dan): Node ile web build → Go build (embed) → alpine, root olmayan `mumweb` kullanıcısı,
  `HEALTHCHECK /healthz`.
- `deploy/docker-compose.yml` (Bölüm 13'e göre) + `deploy/Caddyfile` + `deploy/.env.example`. Plandan farklar:
  - `container_name`'ler kaldırıldı (gereksiz; aynı sunucudaki mevcut `mumble-server`/`caddy` container'larıyla çakışıyordu).
  - Mumble: `MUMBLE_CONFIG_AUTOBAN_SUCCESSFUL_CONNECTIONS=false` (bkz. Faz 2 bulgusu); karşılama metni ve kullanıcı
    sınırı `.env`'den değiştirilebilir.
  - mumweb: `MUMWEB_TRUST_PROXY=true` (Caddy arkasında gerçek istemci IP'si), `MUMWEB_ALLOWED_ORIGINS=https://${MUMWEB_DOMAIN}`.
  - Caddy: HTTP/3 için `443/udp` da açık.
- README (Türkçe): kurulum, DNS, autoban açıklaması, mevcut ters proxy (Caddy/nginx örnekleri) veya mevcut Mumble
  sunucusuyla kullanım, yapılandırma tablosu, güncelleme/yedekleme, sorun giderme, geliştirme.
- mumprobe ve `mumble.Client.ServerFingerprint()`: sunucu sertifikasının SHA-256 parmak izini
  `MUMWEB_TLS_FINGERPRINT` biçiminde yazdırır.

### Doğrulama
- Production compose, ayrı proje adıyla (`mumweb-prodtest`) yerelde ayağa kaldırıldı; override ile yalnızca portlar
  değiştirildi ve gerçek alan adı olmadığı için Caddy'ye `tls internal` verildi (alan adı `ses.test`):
  - 3 servis ayakta, mumweb `healthy`, uid 100 (`mumweb`); Mumble ini'sinde autoban/karşılama/kullanıcı ayarları uygulanmış.
  - `https://ses.test/healthz` → 200 (HTTP/2). `/ws` yanlış Origin → 403, doğru Origin → 101.
  - Headless Chromium (bayraksız, gerçek HTTPS güvenli bağlam): iki tarayıcı oturumu Caddy/TLS üzerinden bağlandı,
    konuşan sayfanın 1 kHz tonu dinleyen sayfada duyuldu (-16 dB), service worker `activated`,
    mumprobe gözlemci kullanıcıları ve sesi gördü. Gateway logundaki IP tarayıcının IP'si (Caddy'ninki değil).
  - Test stack'i volume'leriyle kaldırıldı.
- Parmak izi sabitleme: doğru değerle bağlanıyor, bir baytı değiştirilmiş değerle `fingerprint mismatch`.
- **Bekleyen manuel doğrulama:** temiz bir VPS'te gerçek alan adıyla `docker compose up -d` → `https://ses.<alanadi>`
  ve uçtan uca konuşma (Let's Encrypt sertifikası dahil).

## Faz 8 — Test ve doğrulama (2026-10-13)

- Go: `internal/mumble/conn_test.go` eklendi — sahte TLS Mumble sunucusuyla (sunucu gerektirmez): Version/Authenticate
  içeriği, handshake öncesi ve sonrası **bilinmeyen mesaj tiplerinin atlanması**, `ServerSync` + `ServerConfig` sonrası
  `Info()`/durum, olaylar, giden `UserState`/`UDPTunnel` çerçeveleri, deaf ⇒ mute, sunucu kapatınca olay akışının
  hatayla bitmesi, `Reject`, handshake zaman aşımı, parmak izi uyuşmazlığı, `Close` sonrası goroutine'lerin bitmesi.
  (İlk sürümde parmak izi testi sahte sunucu TLS el sıkışmasını sürmediği için yanlış sebeple — zaman aşımıyla —
  geçiyordu; düzeltildi ve hata metni de doğrulanıyor.)
- Mevcut testler: framing, birleştirme, ses encode/decode, sertifika, köprü ikili çerçeveleri ve sınırlar,
  gerçek sunucuya karşı entegrasyon; Vitest 19 test (ikili çerçeve, jitter buffer, çerçeveleme/yeniden örnekleme,
  VAD, bitrate, PEM, backoff).
- README'ye test komutları ve Bölüm 14 manuel kabul listesi eklendi.

### Kaynak kullanımı (manuel test 12'nin otomatik karşılığı)
Dev stack, 8 çekirdekli makine. 10 WebSocket istemcisi aynı kanalda, **hepsi aynı anda sürekli konuşuyor**
(her biri 50 paket/s; en kötü durum): gateway'e 496 paket/s giriş, 4460 paket/s çıkış (beklenen 4461/s, kayıp yok).

| | Boşta | 10 oturum, 10 konuşan |
|---|---|---|
| gateway CPU | %0 | tek çekirdeğin %40–46'sı |
| gateway RAM | 4.9 MiB | ~15 MiB |
| mumble-server CPU / RAM | – | ~%15 / 6.7 MiB |

Gerçekte aynı anda 1–2 kişi konuşur; yük konuşan sayısı × dinleyen sayısıyla doğru orantılıdır.
Gerekirse ilk optimizasyon adayı: ses çerçevesi başına `context.WithTimeout` + WS yazımı.

## Ek — Yerel HTTPS test ortamı (2026-10-13)

- `deploy/local/compose.yaml` (Caddy + Mumble + mumweb), `deploy/local/Caddyfile` (mkcert sertifikası `certs/`,
  `auto_https disable_certs`), `.env.example` (alan adı, portlar, SuperUser şifresi); `.env` olmadan da çalışır.
  `certs/` içeriği git'e ve Docker build context'ine girmez. Windows checkout'larında LF için `.gitattributes`.
- Gateway: izinli Origin karşılaştırması normalize edildi (küçük harf, sondaki `/`, varsayılan `:443`/`:80`
  yok sayılır); böylece compose `https://<alan>:<HTTPS_PORT>` yazar ve hem 443'te hem başka portta çalışır.
- `docs/DEVELOPMENT.md`: Windows için adım adım rehber (mkcert, hosts, başlatma, masaüstü istemci, port çakışması,
  LAN/Android, sorun giderme); README'den bağlantı verildi.

### Doğrulama (Linux, mkcert yerine openssl ile aynı yapıda CA + sertifika)
- 80/443/64738 bu makinede dolu olduğu için `.env` ile 8444/8082/64741 kullanıldı (port değiştirme yolu da sınandı).
- Caddy ACME'ye gitmeden dosya sertifikasını sundu; `curl --cacert rootCA.pem https://mumweb.test:8444/healthz` → 200.
- Headless Chromium (`mumweb.test` → host IP): güvenli bağlam, iki oturum arası ses (1 kHz, -16 dB), service worker,
  Origin `https://mumweb.test:8444` kabul edildi; mumprobe `localhost:64741` üzerinden kullanıcıları ve sesi gördü.
- Ortam `down -v` ile kaldırıldı, sahte sertifikalar silindi.
- **Bekleyen:** gerçek Windows + Docker Desktop + mkcert üzerinde deneme.

## Ek — Kullanıcı geri bildirimi: sağ tık menüsü ve mikrofon hataları (2026-10-13)

Windows + Firefox'ta yerel testte: "mikrofon kullanılamıyor" uyarısı ve sağ tıkta Firefox menüsünün açılması.
- **Sağ tık menüsü** (`ContextMenu.svelte`): kullanıcı → yerel ses seviyesi (0–200 %), "Benim için sustur", "Sıfırla";
  kendi satırı → mikrofon/ses kapat-aç; kanal → "Kanala katıl" (çift tık da çalışır). Ekran kenarına sığdırılır;
  Esc, dışarı tıklama, kaydırma, pencere değişimi ile kapanır; klavye menü tuşu/Shift+F10 ve Android uzun basma ile açılır.
  Sol tıkla açılan ses paneli kaldırıldı.
- **Mikrofon:** işlemeli (AEC/NS/AGC, mono) `getUserMedia` izin dışı bir hatayla reddedilirse `{audio: true}` ile bir kez
  daha denenir (bazı Windows sürücüleri işlemeli açılışı reddediyor). Uyarı artık hata türüne göre Türkçe açıklama
  (Windows gizlilik ayarı, başka uygulama, cihaz yok…) ve teknik hata adını gösterir; "Tekrar dene" butonu eklendi.
- Doğrulama: geçici Firefox 156 container'ı (Selenium) — sahte mikrofonla normal açılış; Firefox'u farklı örnekleme
  hızına zorlayınca da sorun yok (kullanıcıdaki hata buna bağlı değil); işlemeli istek `NotReadableError` ile
  reddedilince sade istekle mikrofon açıldı; iki istek de reddedilince Türkçe açıklama + `(NotReadableError: …)`,
  "Tekrar dene" sonrası mikrofon açıldı. Menü: `contextmenu` olayı `defaultPrevented` (Firefox menüsü açılmıyor),
  WebDriver sağ tıklama ile kullanıcı/kanal/kendi menüleri, %50 → rozet "50%", yerel susturma → "0%" ve kalıcı,
  menüden kanala katılma Mumble'a yansıdı, menüden mute Mumble'a yansıdı, Esc/dışarı tıklama kapatıyor, sol tık açmıyor.
- **Bekleyen:** kullanıcının Windows makinesinde gerçek hata adının görülmesi (yeni uyarı gösterecek).

## Ek — SuperUser anahtarı, web'den kanal oluşturma, yerel Mumble sertifikası (2026-10-13)

- Gateway `MUMWEB_ALLOW_SUPERUSER` (varsayılan **false**): kapalıyken `SuperUser` adı (büyük/küçük harf duyarsız)
  Mumble'a bağlanılmadan `reject SuperUserDisabled` ile reddedilir. Yerel ve dev compose'ta açık, production'da kapalı.
- Protokol: `{t:"createChannel", parent, name, temporary}` → Mumble `ChannelState`. Yetkiyi sunucunun ACL'i belirler;
  ret `denied` olarak sohbete düşer. Arayüz: kanal sağ tık → "Alt kanal oluştur…" penceresi (ad, geçici kanal).
  Not: plan Bölüm 1 yönetim işlevlerini kapsam dışı sayıyordu; kullanıcı isteğiyle yalnızca kanal oluşturma eklendi.
- Yerel ortamda Mumble sunucusu da mkcert sertifikasını kullanır (`sslCert`/`sslKey`). mkcert anahtarı 600 izinli
  olduğundan ve Mumble uid 10000 ile çalıştığından tek seferlik `certs` servisi dosyaları uygun sahiplikle bir volume'e
  kopyalar (bind mount ile Linux'ta "Failed to read /certs/key.pem" ile kapanıyordu). Sertifika yoksa Türkçe hata ile durur.
- Doğrulama: Go testleri (SuperUser reddi 3 yazımda; kayıtsız kullanıcı kalıcı kanal → `Permission`; SuperUser geçici
  kanal oluşturuyor). Yerel stack (openssl ile mkcert benzeri CA): `openssl s_client -CAfile rootCA.pem
  -verify_hostname mumweb.test` → Mumble portunda `Verify return code: 0`. Tarayıcı: SuperUser web'den girdi (kayıtlı
  ikonu), sağ tık → "Oyun Odası" oluşturuldu (gözlemci istemci de gördü), menüden katılındı; kayıtsız kullanıcı kalıcı
  ve geçici kanal denemesinde "Bu işlem için yetkiniz yok." (Mumble varsayılan ACL'i).

## Ek — Masaüstü Mumble'daki kullanıcı sağ tık menüsü (2026-10-13)

Kullanıcının paylaştığı ekran görüntüsündeki (SuperUser olarak başka kullanıcıya sağ tık) 18 öğe, masaüstü istemcinin
sırasıyla ve izin mantığıyla (`MainWindow::qmUser_aboutToShow`, `updateMenuPermissions`, v1.5.939) uygulandı.
- İzinler: kök izinler `ServerSync.permissions` (`ready.permissions`), kanal izinleri `PermissionQuery` (istek üzerine,
  `flush` ile sıfırlanır). Sunucu SuperUser'a izin bilgisi göndermez; SuperUser = `All & ~(Speak|Whisper)`
  (`ChanACL::effectivePermissions`). Tarayıcıya `userId`, `hash`, `hasComment`, `hasTexture` eklendi.
- Gateway komutları: `moveUser`, `setUser` (mute/deaf/prioritySpeaker), `kick`, `ban`, `register`, `resetComment`,
  `resetTexture`, `requestComment` (RequestBlob), `userStats` (UserStats → `userStats` mesajı), özel `text`
  (`session`), `queryPermissions`.
- **Önemli bulgu — ban:** sunucunun `msgUserRemove(ban)`'ı kullanıcının IP'sini de yasaklar; tüm web kullanıcıları
  gateway IP'sini paylaştığı için bu hepsini dışarıda bırakır. Web'deki "Yasakla" bu yüzden ban listesini çekip
  `100::/128` (RFC 6666, hiçbir istemciyle eşleşmez) adresli + sertifika hash'li bir kayıt ekleyip geri yazar ve
  kullanıcıyı kovar. Masaüstünden yapılan ban'lar için README'ye uyarı eklendi.
- Yerel tercihler (sertifika hash'ine göre, yoksa ada göre): rumuz (ağaçta rumuz, ipucunda gerçek ad), mesajları
  görmezden gel (sohbete düşmez), arkadaş (kalp ikonu; ad değişirse "güncelle"). İmtiyazlı konuşmacı ikonu eklendi.
- Pencereler: kov/yasakla sebebi, rumuz, özel mesaj (Ctrl+Enter), açıklama (DOMPurify), bilgi (istemci, OS,
  süreler, adres, ping, sertifika, paketler).
- SuperUser ile girince sohbette "SuperUser konuşamaz" bilgisi.
- Doğrulama: Go entegrasyon testi `TestAdminActions` (normal kullanıcıyla izin sorgusu, sustur/sağır/imtiyazlı,
  taşı ve geri, özel mesajın yalnız hedefe gitmesi, bilgi, kaydet, kov; ban sonrası yasaklı sertifika geri giremiyor,
  **başka web kullanıcısı girebiliyor**; sunucu logu `Host: "[100:]"`). Tarayıcı (Chromium, iki ayrı bağlam):
  menü içeriği masaüstüyle aynı sırada; taşı, sustur, sağır et, imtiyazlı (ikon), rumuz, özel mesaj iki tarafta,
  görmezden gelinen mesaj görünmüyor, bilgi penceresi, açıklama görüntüle (`<script>` temizlendi) ve sıfırla (sonra
  pasif), arkadaş (kalp), kaydet (kayıtlı), normal kullanıcının menüsünde yönetim öğeleri yok/pasif, kov → hedefte
  "Sunucudan atıldınız: test bitti".
- Bilinen sınır: kendi açıklamasını düzenleme ve avatar gösterme yok (avatar yalnızca kaldırılabiliyor).

## Ek — Masaüstü Mumble'daki kanal sağ tık menüsü (2026-10-13)

Kullanıcının ekran görüntüsündeki kanal menüsü, masaüstü istemcinin sırası ve kurallarıyla
(`qmChannel_aboutToShow` + `updateMenuPermissions`, v1.5.939; iki kural kümesi birlikte uygulanır):
Kanala katıl, Kanalı dinle | Ekle…, Düzenle…, Kaldır… | Bağla, Bağlantıyı kaldır, Tümünün bağlantısını kaldır |
URL'i kopyala, Mesaj gönder… | Filtrelerken gizle, Filtrelerken iğnele.
- Gateway: `editChannel` (ad/açıklama/sıra/kullanıcı sınırı), `removeChannel`, `linkChannels` (bağla/kaldır her zaman
  kendi kanalımız üzerinden, masaüstü gibi), `listen` (`listening_channel_add/remove`), `requestDescription`
  (RequestBlob), `text` + `channelId` + `tree` (`tree_id`). Kanal mesajına `hasDescription`, `maxUsers`; kullanıcıya
  `listening` eklendi.
- **Düzeltme:** Mumble kanal bağlantıları çift yönlüdür (`Channel::link`) ama sunucu değişikliği tek kanal üzerinden
  bildirir; durum artık karşı kanalı da günceller ve iki kanal için de olay yayınlar (birim + entegrasyon testi).
  Kanal silinince diğer kanalların bağlantılarından ve dinleyici listelerinden de düşer.
- URL'i kopyala: masaüstünün `mumble://` adresi yerine (Mumble'ın dış adresi gateway'de bilinmiyor) web adresi
  `…/#kanal=/Yol/Alt`; bu bağlantıyla giriş yapan kişi ilk `ready`'de o kanala katılır.
- Filtre: kanal listesinin başlığında huni düğmesi; açıkken kullanıcısı (veya dinleyicisi) olmayan kanallar gizlenir,
  "iğnele" her zaman gösterir, "gizle" (kendi kanalımız değilse) her zaman gizler. Kanal başına ayarlar yerel.
- Ağaç: dinleyiciler kulak ikonuyla kanalın altında, bağlı kanallarda zincir ikonu. Gönderilen kanal/özel mesajlarda
  "Sen → #Kanal" / "Sen → Ad" etiketi (yalnızca özel mesajlar "özel" işaretli).
- Doğrulama: Go `TestChannelActions` (oluştur, düzenle, açıklama, bağla/kaldır ve tarayıcıya giden iki tarafın sırası,
  dinleme ve `listening` alanı, ağaç mesajı, sil), `TestLinksAreSymmetric`, `TestListeningChannels`. Tarayıcı
  (SuperUser + ayrı bağlamda normal kullanıcı + bağlantıyla giren üçüncü kişi): menü içeriği, ekle, düzenle (form
  yeniden açılınca değerler geri geliyor), bağla/kaldır/tümünü kaldır ikonları, dinleyici satırı ve onay işareti,
  alt kanaldaki kullanıcıya ağaç mesajı, URL kopyala → yeni oturum doğrudan o kanalda, filtre + iğnele/gizle, kaldır;
  normal kullanıcının menüsünde yönetim öğeleri pasif.
- Bilinen sınır: kanal şifresi (ACL ile) ve kanal açıklamasını görüntüleme yok.

## Ek — Şifreli kanallar (2026-10-13)

Mumble'da kanal şifresi bir ACL'dir (masaüstü `ACLEditor::updatePasswordACL`): `@all` için Enter|Speak|Whisper|
TextMessage|LinkChannel|Traverse reddedilir, `#<şifre>` grubuna izin verilir; şifre "erişim token'ı" olarak gönderilir.
Masaüstü 1.5 girerken şifre sormaz (Erişim tokenları penceresi); web'de istenen akış uygulandı.
- Giriş: kilitli kanala (sunucunun `is_enter_restricted` / `can_enter` alanları) katılırken veya "Enter" reddi
  gelince şifre penceresi. Şifre oturumun **kalıcı** erişim token'larına eklenir (oturum ortasında `Authenticate.tokens`;
  sunucu tüm kanalların `can_enter`'ını yeniden gönderir), ardından katılma isteği gider. Geçici token
  (`UserState.temporary_access_tokens`) yalnızca giriş kontrolünde geçerli olduğu için kullanılmadı: şifre ACL'i
  Speak/TextMessage'ı da kısıtlar, kullanıcı içeride konuşamazdı. Yanlış şifre → token geri alınır, pencere hata ile
  tekrar açılır. "Bu tarayıcıda hatırla" → localStorage, girişte `hello.tokens` ile gönderilir; Ayarlar'dan silinir.
- Yönetici: Düzenle penceresinde "Kanal şifresi" (ACL okunup gösterilir; değişince ACL masaüstüyle aynı kurallarla
  geri yazılır: miras alınanlar ve değişmemiş miras gruplar atlanır, mevcut şifre ACL'i + eşleşen `@all` reddi
  değiştirilir).
- Ağaç: kilitli kanal kırmızı kilit, şifresi olunan kanal açık kilit.
- **Bulgu — sunucu mesaj hız sınırı:** murmur kullanıcı başına `messagelimit` (1/sn) / `messageburst` (5) uygular
  (kendi UserState'i, kanal oluşturma, TextMessage, ACL) ve fazlasını sessizce düşürür. ACL yazma 2 mesajdır
  (sorgu + yazma). Testte hızlı ardışık işlemler bu yüzden düştü; README'ye not eklendi.
- Doğrulama: Go `TestChannelPassword` (şifre koy/oku, normal kullanıcıda kilitli + reddedilir, yanlış token
  reddedilir, doğru token → `canEnter` true ve giriş, `hello.tokens` girişten itibaren geçerli, şifre kaldırılınca
  kısıt kalkar). Tarayıcı: SuperUser Düzenle'den şifre koydu; normal kullanıcı kırmızı kilit gördü, çift tıkta şifre
  penceresi, yanlış şifre → hata satırı, doğru şifre + hatırla → kanalda ve sohbet yazabiliyor, sayfa yenilenince
  şifresiz girdi, Ayarlar'dan şifre silinince yeniden kilitli; yönetici Düzenle'de mevcut şifreyi görüyor.

## Ek — Aynı IP'den gelen web kullanıcıları: takma ad devralma açığı (2026-10-13)

Kaynak (murmur `msgAuthenticate`, "Allow reuse of name from same IP"): kayıtsız bir kullanıcı, kullanımdaki bir adla
**aynı IP'den** gelirse sunucu UsernameInUse demek yerine eski oturumu atar. Tüm web kullanıcıları gateway IP'sini
paylaştığı için herhangi bir web kullanıcısı başka bir web kullanıcısının adını yazarak onu düşürebiliyordu.
- Gateway, açık oturumların adlarını (büyük/küçük harf duyarsız) ve sertifika hash'lerini tutar; aynı adla **farklı
  sertifikadan** gelen girişi Mumble'a bağlanmadan `reject UsernameInUse` ile reddeder. Aynı sertifika (yeniden
  bağlanma, ikinci sekme) eskisi gibi kabul edilir ve sunucu eski oturumu kapatır.
- Hash, Mumble'ınkiyle aynı biçimde hesaplanır (DER sertifikanın SHA-1'i, hex); entegrasyon testinde sunucunun
  gördüğü hash ile karşılaştırıldı.
- Doğrulama: `TestNameRegistry` (talep, büyük/küçük harf, aynı sertifikayla ikinci oturum, bırakma) ve
  `TestNameTakeover` (gerçek sunucu: hash eşleşmesi, başka sertifikayla aynı ad reddedildi ve sahibi bağlı kaldı,
  aynı sertifikayla yeniden bağlanma eski oturumu değiştirdi).
- Masaüstü kullanıcıları etkilenmez (farklı IP → sunucu zaten reddeder). IP'nin sunucuda kullanıldığı diğer yerler:
  ban (açık konu), autoban (çözüldü), UserStats adresi (kozmetik).

## Faz 9 kararı (2026-10-13)
WASM Opus yedeği **yapılmayacak** (kullanıcı kararı). Gerekçe: WebKit kaynağına göre Safari 26+ (iOS 26 / macOS)
WebCodecs `AudioEncoder`/`AudioDecoder` ile mono/stereo ham Opus'u destekliyor (`AudioEncoderCocoa.cpp`,
`AudioDecoderCocoa.cpp`); yedek yalnız eski Safari için gerekirdi ve tarayıcıda Opus kodlayıcı kütüphaneleri eski.
Öncelik Android ve masaüstü web özellikleri. Safari 26+ gerçek cihazda test edilmedi.

## Açık konular
- Masaüstünden bir web kullanıcısı yasaklanınca ban kaydı gateway IP'sini de içerir ve yeni web girişlerini engeller
  (mevcut oturumlar düşmez). Seçenekler kullanıcıya sunuldu: (1) yasaklama yetkili "bekçi" hesabıyla otomatik
  düzeltme, (2) yalnızca algılama/uyarı, (3) kural. Karar bekleniyor.
- Production'da Mumble autoban ayarı (Faz 7): en az `autobanSuccessfulConnections=false`. Başarısız girişler
  (yanlış şifre) hâlâ gateway IP'si adına sayılır; 120 sn'de 10 yanlış şifre tüm web kullanıcılarını 5 dk keser.
- Plan 5.3'teki `frame_number` "+1 / 20 ms" ifadesi resmî istemciye göre "+2 / 20 ms" (10 ms birim) olarak
  düzeltilmeli (bkz. Faz 4 notu).
