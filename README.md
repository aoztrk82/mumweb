# mumweb

Tarayıcıdan [Mumble](https://www.mumble.info/) sunucusuna bağlanan web istemcisi. Mumble yüklü olmayan
arkadaşlar bir adresi açıp takma adlarını yazarak sesli sohbete katılır.

```
 Tarayıcı (Svelte SPA)            Docker ağı
 ┌──────────────────────┐  WSS  ┌──────────────┐  TLS/TCP  ┌───────────────┐
 │ UI + AudioWorklet    │ ────► │ mumweb       │ ────────► │ mumble-server │
 │ WebCodecs Opus       │ ◄──── │ gateway (Go) │ ◄──────── │ :64738        │
 └──────────────────────┘       └──────────────┘           └───────────────┘
            ▲                          ▲
            └──── HTTPS ── Caddy ──────┘
```

- Gateway her tarayıcı oturumu için Mumble'a **ayrı, normal bir istemci bağlantısı** açar; sunucu web
  kullanıcılarını masaüstü kullanıcılarından ayırt etmez. Ses Mumble TCP bağlantısındaki `UDPTunnel` ile taşınır.
- Opus kodlama/çözme tarayıcıda (WebCodecs) yapılır; gateway sesi çözmez, CPU açısından hafiftir.
- Gateway yalnızca yapılandırılmış tek Mumble sunucusuna bağlanır (tarayıcı adres seçemez).
- Her tarayıcı kalıcı bir istemci sertifikası (IndexedDB) kullanır; böylece kullanıcılar Mumble'da kendini
  kaydedebilir ve yetki alabilir. Gateway sertifikaları diskte saklamaz.

**Özellikler:** takma ad + isteğe bağlı sunucu şifresi, kanal ağacı ve kanal değiştirme (çift tık), konuşan göstergesi,
masaüstü Mumble'daki gibi sağ tık menüleri — kullanıcı: kanalına katıl, kendi kanalına taşı, kov, yasakla,
sustur, sağır et, imtiyazlı konuşmacı, yerel sustur, mesajları görmezden gel, yerel ses seviyesi (%0–200), rumuz,
açıklamayı görüntüle/sıfırla, avatarı kaldır, özel mesaj, bilgi, kaydet, arkadaş ekle (yönetim öğeleri sunucudaki
yetkinize göre etkin olur); kanal: katıl, alt kanal oluştur; kendin: mute/deafen —
bas-konuş (ayarlanabilir tuş, mobilde büyük buton) ve ses algılama (ayarlanabilir eşik), mute/deafen, kanal sohbeti (HTML temizlenir), otomatik yeniden bağlanma, giriş/çıkış sesleri, koyu/açık tema,
sertifika dışa/içe aktarma, ana ekrana eklenebilir (PWA).

**Tarayıcılar:** Chrome/Edge ve Firefox (masaüstü), Chrome (Android). Safari/iOS henüz desteklenmiyor
(WebCodecs Opus eksik; bkz. plan Faz 9).

> **Kendi bilgisayarınızda denemek mi istiyorsunuz?** Windows (ve macOS/Linux) için mkcert sertifikası ve hosts
> dosyasıyla `https://mumweb.test` üzerinde çalışan hazır ortam: **[docs/DEVELOPMENT.md](docs/DEVELOPMENT.md)**.

## Kurulum (yeni bir sunucuda)

Gereksinimler: Docker + Docker Compose, 80/443 (TCP) ve 64738 (TCP+UDP) portları açık bir Linux sunucu,
bir alan adı.

1. **DNS:** `ses.alanadiniz.com` için sunucunun IP'sine işaret eden bir `A` (ve varsa `AAAA`) kaydı ekleyin.
   Kayıt yayılmadan Caddy sertifika alamaz (`dig +short ses.alanadiniz.com` sunucu IP'sini göstermeli).
2. **Kodu alın ve yapılandırın:**
   ```sh
   git clone <repo-adresi> mumweb && cd mumweb/deploy
   cp .env.example .env
   # .env içinde MUMWEB_DOMAIN ve MUMBLE_SUPERUSER_PASSWORD'ü değiştirin
   ```
3. **Başlatın:**
   ```sh
   docker compose up -d --build
   docker compose ps          # mumweb "healthy" olmalı
   docker compose logs -f caddy   # ilk açılışta sertifika alınmasını izleyin
   ```
4. `https://ses.alanadiniz.com` adresini açın. Masaüstü Mumble istemcileri aynı sunucuya
   `ses.alanadiniz.com:64738` ile bağlanabilir.

Compose dosyası üç servis çalıştırır: `mumble` (resmî `mumblevoip/mumble-server`), `mumweb` (gateway + arayüz,
root olmayan kullanıcı, `/healthz` sağlık kontrolü) ve `caddy` (otomatik HTTPS, WebSocket'i olduğu gibi aktarır).

### Mumble autoban ayarı (önemli)

Tüm web kullanıcıları Mumble'a gateway'in **tek IP'sinden** bağlanır. Mumble'ın varsayılan autoban'ı
(120 sn içinde 10 bağlantı denemesi → 5 dk ban, başarılı bağlantılar dahil) birkaç web girişinden sonra **tüm web
kullanıcılarını** 5 dakika kilitler. Compose dosyası bu yüzden `autobanSuccessfulConnections=false` ayarlar
(yalnızca başarısız denemeler sayılır). Başarısız denemeler (yanlış şifre) yine gateway IP'si adına sayılır;
gateway kendi tarafında IP başına dakikada 10 yeni bağlantıya izin verir.

### Yasaklama (ban) ve web kullanıcıları (önemli)

Mumble'ın kendi "Yasakla" işlemi kullanıcının sertifikasıyla birlikte **IP adresini** de yasaklar. Web kullanıcıları
sunucuya gateway'in IP'sinden bağlandığı için **masaüstü istemciden bir web kullanıcısını yasaklamak tüm web
kullanıcılarını yasaklar.** mumweb arayüzündeki "Yasakla…" bu yüzden yalnızca sertifikayı yasaklar (ban listesine
`100::/128` adresli, hiçbir istemciyle eşleşmeyen bir kayıtla). Masaüstünden yanlışlıkla gateway IP'si yasaklanırsa
masaüstü istemcide *Sunucu → Yasak listesi* üzerinden o kaydı silin. Not: sertifika yasağı, kişi yeni bir sertifika
oluşturursa (tarayıcı verisini silerse) aşılabilir.

`SuperUser` hesabı Mumble'da tasarım gereği konuşamaz (yalnızca yönetim içindir). Web'den SuperUser girişi
`MUMWEB_ALLOW_SUPERUSER` ile açılır; production'da kapalıdır.

## Mevcut bir kurulumla kullanım

### 80/443'te zaten bir ters proxy (Caddy, nginx, Traefik…) varsa

`caddy` servisini kaldırın, `mumweb` servisini proxy'nin erişebileceği ağa bağlayın ve proxy'ye bir site ekleyin.
WebSocket yükseltmesinin (`/ws`) aktarılması gerekir. `MUMWEB_TRUST_PROXY=true` kalsın (proxy `X-Forwarded-For`
eklemelidir) ve `MUMWEB_ALLOWED_ORIGINS` sitenin tam adresi olsun.

Caddy:
```
ses.alanadiniz.com {
	encode zstd gzip
	reverse_proxy mumweb:8080
}
```

nginx:
```nginx
location / {
    proxy_pass http://mumweb:8080;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_read_timeout 120s;
}
```

### Zaten çalışan bir Mumble sunucusu varsa

`mumble` servisini kaldırın ve `MUMWEB_MUMBLE_ADDR`'i mevcut sunucuya yönlendirin (ör. aynı Docker ağındaysa
`mumble-server:64738`). O sunucuda da autoban ayarını yapın: resmî Docker imajında
`MUMBLE_CONFIG_AUTOBAN_SUCCESSFUL_CONNECTIONS=false`, `mumble-server.ini`'de `autobanSuccessfulConnections=false`.

## Yapılandırma (gateway ortam değişkenleri)

| Değişken | Varsayılan | Açıklama |
|---|---|---|
| `MUMWEB_LISTEN` | `:8080` | HTTP dinleme adresi |
| `MUMWEB_MUMBLE_ADDR` | `mumble:64738` | Hedef Mumble sunucusu (tek ve sabit) |
| `MUMWEB_TLS_VERIFY` | `false` | Mumble sunucu sertifikasını doğrula (kendinden imzalıysa kapalı kalmalı) |
| `MUMWEB_TLS_FINGERPRINT` | boş | Doluysa sunucu sertifikasının SHA-256 parmak izi sabitlenir |
| `MUMWEB_MAX_SESSIONS` | `50` | Eşzamanlı web oturumu sınırı |
| `MUMWEB_ALLOWED_ORIGINS` | boş | Virgülle ayrılmış izinli `Origin`'ler (ör. `https://ses.alanadi.com`; varsayılan port yazılsa da olur); boşsa yalnızca aynı host |
| `MUMWEB_CONN_RATE` | `10` | IP başına dakikada yeni bağlantı sınırı (`0` = kapalı) |
| `MUMWEB_TRUST_PROXY` | `false` | İstemci IP'sini `X-Forwarded-For`'dan al (ters proxy arkasında `true`) |
| `MUMWEB_ALLOW_SUPERUSER` | `false` | Web'den Mumble `SuperUser` hesabıyla girişe izin ver (kapalıyken bu ad reddedilir; yönetim masaüstü istemciden) |
| `MUMWEB_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

Mumble sunucusunun parmak izini `mumprobe` yazdırır (`make gateway` sonrası):
`./bin/mumprobe -addr <sunucu>:64738 -exit` → `server certificate SHA-256: AB:CD:…`. Bu değer
`MUMWEB_TLS_FINGERPRINT`'e olduğu gibi yazılabilir. Not: sunucu sertifikası yenilenirse parmak izi değişir.

## Güncelleme

```sh
cd mumweb && git pull
cd deploy && docker compose up -d --build
docker image prune -f   # eski imajları temizle (isteğe bağlı)
```

Mumble sunucusunu güncellemek için: `docker compose pull mumble && docker compose up -d mumble`.
Kullanıcı kayıtları ve kanallar `mumble-data` volume'ünde durur; yedek için:
`docker run --rm -v mumweb_mumble-data:/d -v "$PWD":/b alpine tar czf /b/mumble-data.tgz -C /d .`

## Sorun giderme

| Belirti | Olası sebep / çözüm |
|---|---|
| Site açılmıyor, Caddy loglarında ACME hatası | DNS kaydı henüz sunucuyu göstermiyor veya 80/443 kapalı/dolu. `docker compose logs caddy` |
| "Mumble sunucusuna ulaşılamıyor" | `docker compose logs mumble mumweb`; Mumble ayakta mı, `MUMWEB_MUMBLE_ADDR` doğru mu |
| Birkaç girişten sonra herkes bağlanamıyor | Mumble autoban gateway IP'sini banlamış: yukarıdaki autoban ayarını yapın, banı kaldırmak için Mumble'ı yeniden başlatın |
| Sürekli "Yeniden bağlanılıyor" ve gateway logunda `ws origin rejected` | `MUMWEB_ALLOWED_ORIGINS` tarayıcıdaki adresle birebir aynı olmalı (`https://` dahil, sonda `/` yok) |
| Herkes aynı IP'den geliyor gibi görünüyor / hızlıca "too many connections" | Ters proxy arkasında `MUMWEB_TRUST_PROXY=true` olmalı ve proxy `X-Forwarded-For` eklemeli |
| Mikrofon izni istenmiyor | Sayfa HTTPS üzerinden açılmalı (mikrofon ve WebCodecs güvenli bağlam ister) |
| "Tarayıcınız Opus ses kodlamayı desteklemiyor" | Safari/iOS veya eski tarayıcı; güncel Chrome/Edge/Firefox kullanın |
| Kulaklıksız konuşunca karşı taraf yankı duyuyor | Tarayıcının yankı engellemesi her platformda WebAudio çıkışını yakalayamayabilir; kulaklık veya bas-konuş kullanın |
| "Bu takma ad kayıtlı; şifre veya sertifika yanlış" | Ad Mumble'da başka bir sertifikayla kayıtlı. Doğru tarayıcıdan girin veya sertifikayı Ayarlar → Kimlik'ten içe aktarın |
| Sertifika kayboldu (tarayıcı verisi silindi) | Daha önce dışa aktarılan PEM'i Ayarlar → Kimlik → İçe aktar ile yükleyin; yoksa yönetici kaydı yeni sertifikaya taşımalı |

Gateway logları JSON'dur: `docker compose logs -f mumweb`. Şifre, sertifika anahtarı ve sohbet içeriği loglanmaz.

## Geliştirme

**Yerel HTTPS test ortamı** (Caddy + Mumble + mumweb, mkcert sertifikası, `https://mumweb.test`; Windows için adım
adım anlatım): [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) — `deploy/local/` içinde `docker compose up -d --build`.

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
| `deploy/` | Production compose, Caddyfile, `.env.example`, dev compose |
| `deploy/local/` | Yerel HTTPS test ortamı (mkcert), bkz. [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) |

İlerleme ve doğrulama notları: [docs/PROGRESS.md](docs/PROGRESS.md).

## Testler

```sh
make test                                                    # Go birim testleri + Vitest
MUMWEB_TEST_MUMBLE_ADDR=localhost:64738 go test ./internal/bridge   # gerçek sunucuya karşı (gateway/ içinde)
```

- Go: framing encode/decode ve hatalar, kısmi `UserState`/`ChannelState` birleştirme, `MumbleUDP.Audio` encode/decode,
  sahte TLS sunucusuyla istemci (handshake, bilinmeyen mesaj tipleri, reject, zaman aşımı, parmak izi, kapanış),
  sertifika üretimi, köprü ikili çerçeveleri, Origin/oturum/IP sınırları; entegrasyon: WS ↔ Mumble metin, ses,
  durum, sertifika kalıcılığı, goroutine sızıntısı.
- Vitest: ikili ses çerçeveleri, jitter buffer, çerçeveleme/yeniden örnekleme, VAD, bitrate, PEM, yeniden bağlanma süresi.

### Manuel kabul testleri

Masaüstü Mumble istemcisi aynı sunucuya bağlıyken:

1. Chrome masaüstü: giriş → masaüstü Mumble'da kullanıcı görünüyor.
2. Sayfayı yenile → aynı kimlikle (kayıtlıysa kayıtlı ikonu ile) dönüyor.
3. Kanal değiştir, masaüstünde de değişiyor; masaüstünden taşınınca tarayıcıda da güncelleniyor.
4. Sohbet iki yönde; `<script>` içeren mesaj çalışmıyor.
5. Masaüstünden konuş → tarayıcıda duyuluyor; tarayıcıdan PTT ile konuş → masaüstünde duyuluyor,
   bırakınca masaüstündeki konuşma göstergesi hemen sönüyor.
6. VAD modunda sessiz ortamda paket gitmiyor (masaüstünde konuşma göstergesi yanmıyor).
7. Mute/deafen iki tarafta da doğru ikonlarla görünüyor.
8. Kişi bazlı ses seviyesi sadece yerelde etkili.
9. Wi-Fi'yi 10 sn kapat/aç → otomatik yeniden bağlanıyor.
10. Yanlış sunucu şifresi → anlaşılır Türkçe hata.
11. Firefox masaüstü ve Android Chrome'da 1–5 tekrarlanıyor; Android'de ana ekrana ekle → tam ekran açılıyor, bas-konuş çalışıyor.
12. 10 eşzamanlı tarayıcı oturumunda gateway CPU/RAM makul (ölçüm: `docker stats`).

## Lisans

MIT
