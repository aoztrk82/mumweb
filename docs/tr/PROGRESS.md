# İlerleme günlüğü

[English](../PROGRESS.md) | **Türkçe**

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

## Dokümantasyon: ana dil İngilizce (2026-10-13)

- `README.md`, `docs/DEVELOPMENT.md`, `docs/PROGRESS.md`, `docs/TODO.md` ve yeni `docs/TESTING.md` artık İngilizce.
  Türkçe sürümler `README.tr.md` ve `docs/tr/` altında; her sayfa karşılığına bağlantı veriyor.
- README herkese açık bir kitle için yeniden yapılandırıldı (öne çıkanlar, özellik tablosu, tarayıcı desteği, hızlı
  başlangıç, dağıtım, yapılandırma, derleme, test, sorun giderme). Depo içi çalışma bilgileri yeni `CLAUDE.md`'ye taşındı.
- Deploy dosyaları: `deploy/docker-compose.yml` ve `.env.example` içindeki Türkçe karşılama metni varsayılanı ve
  `deploy/local/compose.yaml` içindeki `certs` servisi hata mesajı İngilizce oldu.

## Ek — İngilizce arayüz ve dil düğmesi (2026-10-13)

- `web/src/i18n/en.ts`, `tr.ts` ile aynı yapıda (`Translations` tipiyle; eksik veya fazla anahtar `npm run check`'i
  düşürür). `i18n/index.svelte.ts` reaktif dili (`language`), bileşenlerin içe aktardığı `t` vekilini ve `Intl` /
  sıralama için `locale()`'i tutar. Dil, yeniden yükleme veya yeniden bağlanma olmadan anında değişir.
- Dil seçimi: kayıtlı tercih (localStorage `mumweb.lang`), yoksa `navigator.languages` içindeki ilk İngilizce veya
  Türkçe, yoksa Türkçe (`FALLBACK`; sahibin kararı).
- Arayüz: giriş ekranının sağ üstünde ve durum çubuğunun sağında **EN / TR** düğmesi (`LangSwitch.svelte`); giriş
  öncesi ve sonrası görünür. `<html lang>` ve meta açıklaması seçime uyar.
- Sözlüğe taşınanlar: tuş adları, aria-label'lar, sohbet saat biçimi, kanal/kullanıcı sıralama dili. `index.html` ve
  `manifest.webmanifest` İngilizce (statik oldukları için seçimi izleyemez).
- Doğrulama: `npm run check` 0 hata, `npm test` (19) ve `npm run build` geçti. Elle tarayıcıda denenmedi: bağlıyken
  dil değiştirme ve İngilizce/Türkçe olmayan tarayıcı dili ile yedek davranış.

## Ek — web erişim şifresi (2026-10-13)

- `MUMWEB_ACCESS_PASSWORD` (boş = kapalı). Tarayıcı bunu `hello.accessPassword` olarak gönderir; Mumble sunucu şifresi
  olan `hello.password`'dan ayrıdır. Kontrol oturumda ilk iştir: kullanıcı adı doğrulamasından, sertifikadan ve
  Mumble bağlantısından önce. Reject nedenleri: `AccessPasswordRequired`, `AccessPasswordWrong`, `AccessRateLimited`
  (Mumble tipi değil; arayüz bunları mesaja çevirir).
- Sabit zamanlı karşılaştırma (iki tarafın SHA-256'sı, `subtle.ConstantTimeCompare`). Yanlış deneme 1 sn sonra
  yanıtlanır; bir IP'den 10 dakikada 5 yanlış deneme o IP'yi doğru şifreyle bile kilitler (`failTracker`). Mevcut IP
  başına bağlantı sınırı da geçerli. Şifre loglanmaz.
- `GET /config` `{"accessPassword": bool}` döner (`Cache-Control: no-store`; service worker dışında). Giriş ekranı bunu
  çeker ve "Erişim şifresi" alanını yalnızca gerekirse gösterir; "Bu tarayıcıda hatırla" (localStorage) var. `Access…`
  nedenli bir reject de alanı gösterir, yanlış şifre kayıtlı olanı unutturur.
- Sınırlar: kullanıcı başına kimlik doğrulama değil, "kapı kodu"; masaüstü Mumble istemcileri (64738) bunu atlar; onu
  taşıyan HTTPS bağlantısı kadar gizlidir.
- Doğrulama: Go testleri `TestAccessPassword` (gerekli, yanlış, Mumble sunucu şifresi kabul edilmez, doğru şifre geçer,
  5 yanlıştan sonra doğru şifre de kilitli), `TestFailTracker`, `TestPasswordMatches`, `TestNoAccessPassword`; derlenen
  gateway değişken ayarlıyken `/config`'te `true` döndü. `npm run check` 0 hata, `npm test` ve `npm run build` geçti.
  **Tarayıcıda elle denenmedi.**

## Ek — Android'de ilk gerçek cihaz testi (2026-10-13)

Sahibinin bildirdiği: Android telefon (Samsung Internet) yerel ortama bağlandı ve gerçek mikrofonlarla masaüstü
istemciyle konuşuldu.
- Çalışan: PC ↔ telefon arasında ses ve mesaj iki yönde; mobil görünüm iyi; ekran kilitlenince oturum sürüyor.
- Firefox Android 157.0.1: sayfa codec'in desteklenmediğini söylüyor. Sebep henüz incelenmedi.
- Arka plana alınınca: kullanıcı 15–20 sn sonra odadan düşüyor. Sebep henüz incelenmedi.
- "Ana ekrana ekle" seçeneği çıkmadı; büyük olasılıkla telefon mkcert CA'ya güvenmediği için (sayfa kurulum için
  güvenilir bir güvenli kaynak değil). Gerçek sertifikayla tekrar denenecek.
- Sonuç: "gerçek mikrofonla iki yönde" manuel testi kapandı; yukarıdaki üç sorun TODO'da (B, C, D).

## Ek — yayınlanan Docker imajı (2026-10-13)

- Docker Hub'da `aoztrk82/mumweb` (herkese açık), `latest` ve `0.1.0` etiketleri, `linux/amd64` ve `linux/arm64`
  platformları (index digest'i `sha256:03ce904f…`). Dockerfile web ve Go aşamalarını derleme platformunda
  (`--platform=$BUILDPLATFORM`) çalıştırır ve Go'yu `TARGETOS`/`TARGETARCH`'tan gelen `GOOS`/`GOARCH` ile çapraz
  derler; ağır adımlar için emülasyon gerekmez. `make publish`, `docker buildx build --push` komutunu sarar.
- `deploy/docker-compose.yml` artık derlemek yerine `aoztrk82/mumweb:${MUMWEB_VERSION:-latest}` imajını çeker;
  `deploy/docker-compose.build.yml` kaynaktan derleme override'ıdır. README kurulumunda `git clone` gerekmez: üç dosya
  (compose, Caddyfile, `.env`) listelenir; compose örneği depodaki dosyanın kopyasıdır. Güncelleme
  `docker compose pull && docker compose up -d`.
- Derleme, test, depo yapısı ve sürüm yayınlama adımları README'den `docs/tr/BUILDING.md`'ye taşındı.
- Doğrulama: yayınlanan `0.1.0` imajı Hub'dan çekilip `MUMWEB_ACCESS_PASSWORD` ayarlı çalıştırıldı: `/config` →
  `{"accessPassword":true}`, `/healthz` → `ok`, uid 100 (`mumweb`) ile çalışıyor. Manifest amd64 ve arm64'ü listeliyor.
  **arm64 imajı çalıştırılmadı** (derleme makinesinde emülasyon yok); sorunsuz derlendi. Güncellenen production compose
  yalnızca `docker compose config` ile doğrulandı; yeni README adımlarıyla temiz bir sunucuda tam kurulum henüz yapılmadı.

- Docker Hub sayfası: kısa açıklama ve README (`docs/DOCKERHUB.md`: imajın ne olduğu, etiketler, asgari compose
  parçası, değişken tablosu, tam kurulum için GitHub bağlantıları) Docker Hub API ile
  `scripts/dockerhub-description.py` (`make hub-readme`) üzerinden, Docker Desktop'ın kayıtlı girişi kullanılarak
  ayarlandı. Herkese açık depo sayfası API'sinden kontrol edildi.

## Ek — web yönetim paneli (2026-10-14)

- **Ne:** `/admin/`, gateway'in sunduğu ikinci bir Vite sayfasıdır (`web/admin/index.html`, `web/src/admin/`; `/admin`
  adresi `/admin/`'e yönlendirir). Çevrimiçi kullanıcıları, kanal ağacını ve kanal şifrelerini, tam bir ACL ve grup
  düzenleyicisini, yasak listesini ve kayıtlı kullanıcıları yönetir. `/config` artık `{"admin": bool}` da bildirir.
- **Karar: panel, `hello.role = "admin"` olan normal bir köprü oturumudur.** `MUMWEB_ADMIN_PASSWORD` gerekir (boş =
  panel kapalı, rol `AdminDisabled` ile reddedilir). Yönetici şifresi bu sayfada web erişim şifresinin yerini alır ve
  kendi `failTracker`'ı vardır: `MUMWEB_TRUST_PROXY` olmadan bir proxy arkasında herkes tek IP paylaşır; ortak bir sayaç
  yönetici şifresine kaba kuvvet denemesinin herkesi ana arayüzden kilitlemesine yol açardı. Mumble girişinin önünde
  şifre olmasının nedeni: yabancı biri SuperUser şifresini gateway üzerinden denemeye bile başlayamasın.
- **Karar: SuperUser yalnızca yönetim oturumlarında serbest** (yönetici şifresinden sonra). `MUMWEB_ALLOW_SUPERUSER` ana
  istemci için olduğu gibi kalır, production'da ana giriş kapalı tutulur. "SuperUser konuşamaz" sorusuna da cevap olur:
  panelde ses hiç yok, kimse konuşma beklemez.
- **Karar: ses ve yönetim aynı tarayıcıda yan yana.** Bir tarayıcının iki sekmesi tek IndexedDB sertifikasını paylaşıyordu;
  aynı kayıtlı hesap yeniden bağlanınca Mumble eski oturumu düşürür (ve gateway aynı adı başka sertifikayla reddeder).
  Bu yüzden panel kendi sertifika anahtarını (`cert-admin`) kullanır ve sesli sekmeden farklı bir hesapla (SuperUser)
  girmelidir. Yönetim oturumları girişten hemen sonra sağırlaştırılır, gateway onlar için ses olaylarını atar ve
  onlardan gelen ikili çerçeveyi protokol ihlali sayar.
- **Sunucunun kontrollerine ek gateway kontrolleri:** yönetici olmayan oturumda yönetim komutları (`adminBanList`,
  `adminSetBanList`, `adminUserList`, `adminUpdateUsers`, `adminACL`, `adminSetACL`) oturumu protokol hatasıyla sonlandırır.
  Yasak listesi yazımı yalnızca sunucunun son gönderdiği listedeki kayıtları koruyabilir veya yalnızca sertifikaya bağlı
  kayıt (`100::/128` ve özet) ekleyebilir: yeni bir IP yasağı tüm web kullanıcılarını vurur, ama masaüstünden yapılmış IP
  yasağını silmek serbesttir. SuperUser (kimlik 0) yeniden adlandırılamaz/silinemez; izin maskeleri, grup adları ve liste
  boyutları sınırlıdır; yönetim JSON okuma sınırı 16 KiB yerine 512 KiB'dir (yasak listeleri, ACL'ler).
- **Hız sınırı:** Mumble sunucusu ~1 mesaj/sn (burst 5) üstünü sorgular dahil sessizce atar. Bir saniyede on komut
  gönderen entegrasyon denemesi son sorguyu kaybetti. Yönetim komutları artık gateway'de yavaşlatılır (0,8/sn, burst 4,
  komut en fazla 6 sn bekler, aksi halde panel `adminBusy` alır) ve arayüz her yazmadan sonra taslağına güvenmek yerine
  listeyi yeniden okur.
- **Bayat listeler:** Yasaklar, Kayıtlı kullanıcılar ve ACL görünümleri verilerini önbelleğe almak yerine her açılışta (ve
  kayıttan sonra) yeniden okur; aksi halde Çevrimiçi'den yasaklanan veya kaydedilen kullanıcı eksik görünür, bayat bir
  yasak listesini kaydetmek de daha yeni kaydı silerdi. Kalan yarış (Yasaklar sekmesi açıkken başka yerden yapılan bir
  değişiklik) TODO'da. Yönetici kilidinin kendi ret nedeni var (`AdminRateLimited`).
- **Denetim logu:** durumu değiştiren her yönetim komutu işlem, kanal/hedef kimlikleri ve sayılarla loglanır. Asla şifre,
  sohbet veya ACL grup adı (`#ad` biçimli grup bir kanal şifresidir).
- **Protokol eklemeleri (Go `internal/mumble/admin.go`):** `UserList` hiç bağlı değildi; `BanList` ve `ACL` yanıtları, bekleyen
  bir kick-ban veya şifre isteği yoksa yutuluyordu. Artık `BanListEvent`, `UserListEvent` ve `ACLEvent` üretiyorlar; ACL
  geri yazımı masaüstü istemcinin süzgecini kullanır (kendi devralınmamış kayıtlar, düz devralmadan farklı gruplar).
  Aynı kanalda şifre sorgusu ile ACL raporu artık çakışmaz.
- **Önlenen service worker hatası:** `sw.js` her gezinme yanıtını `/` anahtarıyla saklıyordu; `/admin/` açılınca önbellekteki
  uygulama kabuğu panelle değişirdi (ve çevrimdışı `/admin` sesli uygulamayı gösterirdi). Artık yalnızca `/` önbelleğe alınır,
  `/admin*`'e dokunulmaz ve önbellek adı `mumweb-static-v2`'dir.
- **Doğrulama:**
  - Go birim testleri: sahte TLS Mumble sunucusuyla yasak listesi, kullanıcı listesi ve ACL gidiş-dönüşü (değiştirilmemiş
    geri yazım yalnızca kendi kayıtları üretir), şifre/kilit ayrımı, yasak/ACL/kullanıcı güncelleme doğrulaması, yavaşlatma.
  - SuperUser olarak gerçek bir **Mumble 1.5.915** sunucusuna karşı Go entegrasyon testi `TestAdminPanelEndToEnd`: normal oturum
    yönetim komutu kullanamaz; kullanıcı listesi; yasak ekle/oku/sil; geçici kanalda ACL yazma ve geri okuma (izin/red ve
    `applySubs=false` olan kayıt, üyeli grup); yeni IP yasağı oturumu sonlandırır.
  - Yeniden derlenen yerel kurulumda (`deploy/local`, `https://mumweb.test`) başsız Chrome (puppeteer-core, geçici betikler,
    commit edilmedi): yanlış yönetici şifresi reddedildi; SuperUser girişi; dört sekme de çiziliyor; prompt ile kanal
    oluşturma; ACL düzenleyicisi Root'ta dört bölümüyle açılıyor (admin, auth, all kuralları ve admin grubu); sertifika
    yasağı eklendi, kaydedildi, yeniden okundu ve silindi; konsol hatası yok. Sesli bir oturum (`ses-e2e`) ve panel aynı
    tarayıcıda 6 sn birlikte bağlı kaldı ve panel sesli kullanıcıyı listeledi. Test kanalı ve yasağı sonra silindi.
  - İnceleme düzeltmelerinden sonraki son tur: `go test ./internal/bridge` paketinin tamamı (eski kick/ban, kanal şifresi ve
    ad devralma entegrasyon testleri dahil) yeni bir Mumble 1.5.915 kapsayıcısına karşı; ardından yerel kurulum yeniden
    derlenip tarayıcı betikleri tekrarlandı: **Çevrimiçi**'den sesli kullanıcı kaydedildi (Kayıtlı kullanıcılar'da göründü,
    yeniden silindi) ve sertifika yasağı konuldu (Yasaklar'da göründü, sesli oturumu sonlandı, kayıt silinip kaydedildi).
    Yasaklar / Kayıtlı kullanıcılar sekmeleri işlemden önce açılmıştı; yani bayat önbellek olsaydı görünürdü.
  - Yapılmadı: telefon/dar ekran kontrolü, masaüstü Mumble istemcisinin ACL düzenleyicisinde panelin yazdığı ACL'yi görmek,
    uzun süreli kullanım. Docker Hub sayfası **güncellenmedi** (yayınlanmış imajda panel henüz yok).
- Yerel kurulum: `MUMWEB_ADMIN_PASSWORD` varsayılanı `deploy/local/compose.yaml` ve dev compose dosyasında `admin`'dir.

## Ek — 0.2.0 sürümü (2026-10-14)

- `aoztrk82/mumweb:0.2.0` ve `:latest`, `docker buildx build --builder mumweb-builder --platform linux/amd64,linux/arm64
  --push` ile gönderildi (`make publish`'in sardığı komut; Windows geliştirme makinesinde GNU Make kurulu değil). Index
  digest'i `sha256:c25f8ce3…`. İçerik: yönetim paneli. `main.go`'daki `release` artık `mumweb 0.2`.
- Doğrulama: manifest amd64 ve arm64'ü listeliyor; çekilen imaj `MUMWEB_ADMIN_PASSWORD` ile başlatıldı: `/config` →
  `{"accessPassword":false,"admin":true}`, `/healthz` → `ok`, `/admin` → `/admin/`'e 301 (200), uid 100 ile çalışıyor.
  arm64 imajı yine çalıştırılmadı. Docker Hub sayfası `scripts/dockerhub-description.py` ile yenilendi
  (`MUMWEB_ADMIN_PASSWORD` listeleniyor).

## Ek — güvenlik incelemesi, ilk tur (2026-10-14)

Tam metin [SECURITY.md](SECURITY.md)'de; bu kayıt kararları ve doğrulamayı tutar.

- **Taramalar:** `govulncheck` (aracın projenin Go sürümüyle, `GOTOOLCHAIN=go1.27.2`, derlenmesi gerekir; yoksa Go 1.27 kodunu
  okuyamaz), `npm audit`, yayınlanan `aoztrk82/mumweb:0.2.0` üzerinde Trivy (`--db-repository ghcr.io/aquasecurity/trivy-db:2`
  gerekti, varsayılan ayna 404 verdi): hepsi temiz. `gosec`: 3 kabul edilen bulgu; `staticcheck`: bir testte bilerek tekrarlanan çağrı.
- **Değişiklikler:** gateway'den CSP ve diğer sertleştirme başlıkları (`cmd/mumweb` içinde `securityHeaders`), Caddy'den HSTS;
  IP başına eşzamanlı oturum sınırı (`MUMWEB_MAX_SESSIONS_PER_IP`, 10), çünkü yalnızca IP başına hız yine de tek adresin tüm
  yuvaları doldurmasına izin veriyordu; kontrol mesajı kovası (50/sn, giriş sırasındaki izin sorguları geçsin diye burst 200;
  art arda 500 düşen mesaj oturumu bitirir); HTTP `IdleTimeout` ve `MaxHeaderBytes`; compose sertleştirmesi (gateway salt
  okunur, capability yok, sınırlar, internetsiz `internal` ağ; Caddy `NET_BIND_SERVICE`'i korur; Mumble yalnızca
  `no-new-privileges` ve işlem sınırı).
- **Karar: Mumble capability'lerini korur.** `cap_drop: ALL` giriş betiğinin çıkmasına yol açtı ("Failed to set initial
  capabilities", sonra `/data` izin hatası). Bilerek bırakıldı; SECURITY.md'de kayıtlı.
- **Karar: çıkarılamaz (non-extractable) WebCrypto anahtarı yok.** Gateway TLS istemci sertifikasını kurar, bu yüzden her
  bağlanışta PEM'e ihtiyaç duyar; azaltma sıkı CSP ve XSS testleridir, maruz kalma kabul edilen risktir.
- **Karar: HSTS gateway'de değil Caddy'de** (gateway geliştirmede HTTP ile sunulabilir). Dokümandaki Caddyfile `/admin`'i tek
  adrese kısıtlar; yerel Caddy'de denendi (başka adreslere 404, `/` 200).
- **Doğrulama:**
  - Go: beş fuzz hedefi (`FuzzParseAudioFrame`, `FuzzClientMessage`, `FuzzReadFrame`, `FuzzDecodeAudio`, `FuzzStateMessages`),
    25'er sn, çökme yok; başlıklar, IP başına sınır (HTTP üzerinden de: aynı IP'den üçüncü soket 429 alır) ve gerçek bir
    Mumble 1.5.915'e karşı oturumu bitiren kontrol seli (150 mesajlık giriş patlaması kabul edilirken) için yeni testler.
    Entegrasyon paketinin tamamı yine geçti.
  - Web: sanitizer ve `textToHtml` için 25 XSS birim testi (jsdom, yeni bir geliştirme bağımlılığı).
  - Sertleştirilmiş compose ile yeniden kurulan yerel ortam: başlıklar Caddy üzerinden görüldü (CSP, HSTS, X-Frame-Options, ...);
    gateway container'ı salt okunur (`touch /x` başarısız), çıkış yolu yok (`wget http://1.1.1.1` → ağa ulaşılamıyor),
    `CapDrop ALL`, 256 MiB ve 200 işlemle çalışıyor; başsız Chrome sesli istemciyi (giriş, `<img onerror>` yüklü sohbetin metin
    olarak görünmesi) ve tüm yönetim sekmelerini **konsol veya CSP hatası olmadan** çalıştırdı.
  - Bu Windows makinesinde yerel Mumble container'ı UDP 64738'i yayınlayamıyor (port rezervasyonu); bu yüzden Mumble port
    eşlemesini düşüren geçici bir override ile başlatıldı; o dosya depoda yok.
- Henüz yapılmayanlar: SECURITY.md 7. bölüm (CI taramaları, Semgrep/ESLint, uzun fuzz, digest sabitleme, distroless).

## Ek — 0.3.0 sürümü (2026-10-14)

- Güvenlik ilk turunu yayınlar: güvenlik başlıkları ve CSP, IP başına oturum sınırı (`MUMWEB_MAX_SESSIONS_PER_IP`), kontrol
  mesajı hız sınırı, HTTP boşta/başlık sınırları, sertleştirilmiş compose dosyaları, Caddyfile'larda HSTS. `main.go`'daki
  `release` artık `mumweb 0.3`. Yeni ayar nedeniyle minor sürüm seçildi. `:0.3.0` ve `:latest` gönderildi (amd64+arm64, index digest `sha256:d6657463…`); Trivy: 0 HIGH/CRITICAL; container yeni başlıklarla yanıt veriyor. Sahibi GitHub özel güvenlik açığı bildirimini açtı.

## Ek — ayrılmış web slotları (2026-10-14)

- Yeni `MUMWEB_RESERVED_SLOTS` + `MUMWEB_RESERVED_PASSWORD`: `MUMWEB_MAX_SESSIONS` web slotundan N tanesini yalnızca hello'da
  ayrılmış şifreyi gönderen tarayıcılar (isteğe bağlı giriş alanı, `/config` `reservedSlots` dediğinde görünür) ve yönetim
  oturumları alabilir. Kararlar: toplam sınır soket kabulünde uygulanan sert sınır olarak kalır; slot ayrımı giriş
  kontrollerinden sonra uygulanır (`acquireSlot`), ayrıcalıklı kullanıcılar önce genel slotları doldurur, ayrılmışlar genel
  alan dolunca kullanılır; yanlış ayrılmış şifre sessizce genel slota düşmek yerine reddedilir (ayrı hata sayacı, erişim
  şifresiyle aynı bekleme ve kilit); iki değer de ayarlı ve en az bir genel slot kalıyorsa açılır. Yeni reject nedenleri
  `WebFull`, `ReservedPasswordWrong`, `ReservedRateLimited`. Web sınırı zaten vardı (`MUMWEB_MAX_SESSIONS`, Mumble
  sunucusunun kullanıcı sınırından bağımsız). Birim testleri: `slots_test.go`.

## Ek — 0.3.1 sürümü (2026-10-14)

- Ayrılmış web slotlarını ekler. `aoztrk82/mumweb:0.3.1` ve `:latest` gönderildi (amd64+arm64, index digest `sha256:0e627df0…`);
  Trivy: 0 HIGH/CRITICAL; ayarlıyken imaj `/config` için `reservedSlots:true` döner; Docker Hub sayfası yenilendi.
  Sunucu test dağıtımı (`deploy/test/`) bu imajı çeker.

## Ek — mumweb.aozturk.site üzerinde canlı güvenlik ve yük testi (2026-10-14)

- Çalışan test dağıtımına (0.3.1 sürümü) dışarıdan bir tur kara kutu testi çalıştırıldı: yanıt başlıkları/TLS, açık
  portlar, WebSocket Origin kontrolü, erişim şifresi kilitlemesi (ve 10 dakikalık süresinin dolması), ayrılmış web
  slotları (doğru/yanlış şifre), yönetim şifresi (erişim şifresinden ayrı kilitleme), 120 eşzamanlı denemede IP başına
  oturum sınırı, aşırı büyük/bozuk ilk çerçeveler, kontrol mesajı seli (tasarlandığı gibi 500 düşürmeden sonra oturumu
  bitiriyor), ses çerçevesi seli (tasarlandığı gibi sadece düşürülüyor, oturum açık kalıyor), boşta/hello'suz zaman
  aşımları (slowloris sızıntısı yok), sohbet seli, sohbette ve kullanıcı adı alanında gerçek tarayıcıyla XSS denemesi
  (Mumble'ın kendi sunucusu `<`/`>` karakterlerini gateway veya tarayıcı görmeden reddediyor) ve 60 eşzamanlı tam giriş
  (4 saniyeden kısa sürede tamamlandı, `/healthz` etkilenmedi). Her şey tuttu; kod değişikliği gerekmedi. Ayrıntı ve her
  birinin gerekçesi: [SECURITY.md](SECURITY.md).

## Açık konular
- Masaüstünden bir web kullanıcısı yasaklanınca ban kaydı gateway IP'sini de içerir ve yeni web girişlerini engeller
  (mevcut oturumlar düşmez). **Karar (2026-10-13): şimdilik önlem alınmayacak** — hedef kullanım küçük gruplar/takımlar,
  yasaklama nadir. Web arayüzündeki "Yasakla" yalnızca sertifikayı yasaklar; masaüstünden yasaklama olursa yönetici
  yasak listesinden IP'li kaydı siler (README'de anlatıldı). İleride gerekirse "bekçi hesabı" ile otomatik düzeltme
  seçeneği değerlendirilebilir.
- Production'da Mumble autoban ayarı (Faz 7): en az `autobanSuccessfulConnections=false`. Başarısız girişler
  (yanlış şifre) hâlâ gateway IP'si adına sayılır; 120 sn'de 10 yanlış şifre tüm web kullanıcılarını 5 dk keser.
- Plan 5.3'teki `frame_number` "+1 / 20 ms" ifadesi resmî istemciye göre "+2 / 20 ms" (10 ms birim) olarak
  düzeltilmeli (bkz. Faz 4 notu).
