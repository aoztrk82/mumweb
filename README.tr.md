# mumweb

[English](README.md) | **Türkçe**

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
- İsteğe bağlı `/admin/` yönetim paneli (şifre ayarlanana kadar kapalı): ACL ve grup düzenleyici, yasak listesi, kayıtlı
  kullanıcılar ve kanal yönetimi; aynı tarayıcıda sesli istemcinin yanında çalışır.
- Her tarayıcı kalıcı bir istemci sertifikası (IndexedDB) kullanır; böylece kullanıcılar Mumble'da kendini
  kaydedebilir ve yetki alabilir. Gateway sertifikaları diskte saklamaz.

**Özellikler:** takma ad + isteğe bağlı sunucu şifresi, kanal ağacı ve kanal değiştirme (çift tık), konuşan göstergesi,
şifreli kanallar (kilit ikonu; girerken şifre sorulur, "hatırla" ile sonraki girişlerde otomatik; yönetici
"Düzenle"den şifre koyar/kaldırır), boş kanalları gizleyen filtre, masaüstü Mumble'daki gibi sağ tık menüleri — kullanıcı: kanalına katıl, kendi kanalına taşı, kov, yasakla,
sustur, sağır et, imtiyazlı konuşmacı, yerel sustur, mesajları görmezden gel, yerel ses seviyesi (%0–200), rumuz,
açıklamayı görüntüle/sıfırla, avatarı kaldır, özel mesaj, bilgi, kaydet, arkadaş ekle (yönetim öğeleri sunucudaki
yetkinize göre etkin olur); kanal: katıl, kanalı dinle, ekle, düzenle (ad, açıklama, sıra, kullanıcı sınırı), kaldır,
bağla / bağlantıyı kaldır / tümünün bağlantısını kaldır, URL kopyala (giriş sonrası o kanala götüren web bağlantısı),
mesaj gönder (alt kanallar dahil seçeneğiyle), filtrelerken gizle/iğnele; kendin: mute/deafen —
bas-konuş (ayarlanabilir tuş, mobilde büyük buton) ve ses algılama (ayarlanabilir eşik), mute/deafen, kanal sohbeti (HTML temizlenir), otomatik yeniden bağlanma, giriş/çıkış sesleri, koyu/açık tema,
sertifika dışa/içe aktarma, ana ekrana eklenebilir (PWA).

**Arayüz dili:** İngilizce ve Türkçe. Tarayıcı diline göre açılır (ikisi de değilse Türkçe); giriş ekranında ve durum çubuğunda
daima görünen EN / TR düğmesiyle değiştirilir.

**Tarayıcılar:** Chrome/Edge ve Firefox (masaüstü); Android'de Samsung Internet çalışıyor (masaüstü istemciyle ses ve
mesaj denendi), Android Chrome denenmedi, Firefox Android henüz çalışmıyor ("codec desteklenmiyor", inceleniyor).
Safari/iOS henüz desteklenmiyor (WebCodecs Opus eksik; bkz. plan Faz 9). Android'de bilinen sınır: tarayıcı arka plana
alınınca oturum yaklaşık 15–20 sn sonra odadan düşer (ekran kilidi sorun değil).

> **Kendi bilgisayarınızda denemek mi istiyorsunuz?** Windows (ve macOS/Linux) için mkcert sertifikası ve hosts
> dosyasıyla `https://mumweb.test` üzerinde çalışan hazır ortam: **[docs/tr/DEVELOPMENT.md](docs/tr/DEVELOPMENT.md)**.

## Kurulum (yeni bir sunucuda)

Gereksinimler: Docker + Docker Compose kurulu bir Linux sunucu (x86-64 veya ARM64), 80/443 (TCP) ve 64738 (TCP+UDP)
portları açık, bir alan adı. Derlenecek bir şey yok: hazır imaj Docker Hub'da,
[`aoztrk82/mumweb`](https://hub.docker.com/r/aoztrk82/mumweb).

1. **DNS:** `ses.alanadiniz.com` için sunucunun IP'sine işaret eden bir `A` (ve varsa `AAAA`) kaydı ekleyin.
   Kayıt yayılmadan Caddy sertifika alamaz (`dig +short ses.alanadiniz.com` sunucu IP'sini göstermeli).
2. **Üç dosyalı bir klasör oluşturun:**
   ```sh
   mkdir mumweb && cd mumweb
   ```
   `docker-compose.yml`:
   ```yaml
   # Production stack: Mumble server + mumweb gateway + Caddy (automatic HTTPS).
   # Put MUMWEB_DOMAIN and MUMBLE_SUPERUSER_PASSWORD in a .env file next to this one, then:
   #   docker compose up -d
   name: mumweb

   services:
     mumble:
       image: ${MUMBLE_IMAGE:-mumblevoip/mumble-server:latest@sha256:018ad3515932e513d8fdc970df918373a2664c4da32b09a89a4aaee28eb7507a}
       restart: unless-stopped
       ports:
         # Desktop Mumble clients connect here directly.
         - "64738:64738/tcp"
         - "64738:64738/udp"
       volumes:
         - mumble-data:/data
       environment:
         MUMBLE_SUPERUSER_PASSWORD: ${MUMBLE_SUPERUSER_PASSWORD:?set MUMBLE_SUPERUSER_PASSWORD in .env}
         MUMBLE_CONFIG_WELCOMETEXT: ${MUMBLE_WELCOME_TEXT:-Welcome to our Mumble server!}
         MUMBLE_CONFIG_USERS: ${MUMBLE_MAX_USERS:-50}
         # All web users reach Mumble from the gateway's single IP. The default autoban
         # (10 connections / 120 s, successful ones included) would lock them all out
         # for 5 minutes after a few logins. Count only failed attempts.
         MUMBLE_CONFIG_AUTOBAN_SUCCESSFUL_CONNECTIONS: "false"
       # Hardening: no extra privileges. (cap_drop: ALL breaks the image's entrypoint, which sets
       # capabilities before it drops to its own user.) Reachable by the gateway on the internal network.
       security_opt: ["no-new-privileges:true"]
       pids_limit: 512
       networks: [default, backend]

     mumweb:
       image: ${MUMWEB_IMAGE:-aoztrk82/mumweb:0.3.1@sha256:0e627df0611c6904a30f2d721ed166b566f7f6dda471b54165614443c6a56a3c}
       restart: unless-stopped
       environment:
         MUMWEB_MUMBLE_ADDR: mumble:64738
         MUMWEB_ALLOWED_ORIGINS: https://${MUMWEB_DOMAIN:?set MUMWEB_DOMAIN in .env}
         # Behind Caddy: take the client IP from X-Forwarded-For for per-IP limits.
         MUMWEB_TRUST_PROXY: "true"
         MUMWEB_MAX_SESSIONS: ${MUMWEB_MAX_SESSIONS:-50}
         MUMWEB_LOG_LEVEL: ${MUMWEB_LOG_LEVEL:-info}
         # Logging in as Mumble's SuperUser from the browser. Off by default: the web login
         # is reachable by anyone who knows the URL; use the desktop client for admin work.
         MUMWEB_ALLOW_SUPERUSER: ${MUMWEB_ALLOW_SUPERUSER:-false}
         # Optional password every web user must enter (independent of the Mumble server password).
         MUMWEB_ACCESS_PASSWORD: ${MUMWEB_ACCESS_PASSWORD:-}
         # Switches on the /admin panel (empty = panel off). Separate from the access password.
         MUMWEB_ADMIN_PASSWORD: ${MUMWEB_ADMIN_PASSWORD:-}
         # Keep some of the MUMWEB_MAX_SESSIONS slots for people who know this password (off if either is empty).
         MUMWEB_RESERVED_SLOTS: ${MUMWEB_RESERVED_SLOTS:-0}
         MUMWEB_RESERVED_PASSWORD: ${MUMWEB_RESERVED_PASSWORD:-}
       # Hardening: nothing writable, no capabilities, resource limits, and no way out to the
       # internet: the gateway only sits on the internal network with Mumble and Caddy.
       read_only: true
       cap_drop: [ALL]
       security_opt: ["no-new-privileges:true"]
       mem_limit: 256m
       pids_limit: 200
       cpus: 1.0
       networks: [backend]
       depends_on:
         - mumble

     caddy:
       image: ${CADDY_IMAGE:-caddy:2@sha256:685ae3d7c34d9b5f8caeddc5d14051db7bbd88ff1f5a2fa0c16bd8ca328fb7af}
       restart: unless-stopped
       ports:
         - "80:80"
         - "443:443"
         - "443:443/udp"
       environment:
         MUMWEB_DOMAIN: ${MUMWEB_DOMAIN}
       volumes:
         - ./Caddyfile:/etc/caddy/Caddyfile:ro
         - caddy-data:/data
         - caddy-config:/config
       cap_drop: [ALL]
       cap_add: [NET_BIND_SERVICE]
       security_opt: ["no-new-privileges:true"]
       pids_limit: 200
       networks: [default, backend]
       depends_on:
         - mumweb

   networks:
     # No route to the outside: only Mumble, the gateway and Caddy talk here.
     backend:
       internal: true

   volumes:
     mumble-data:
     caddy-data:
     caddy-config:
   ```
   `Caddyfile`:
   ```
   {$MUMWEB_DOMAIN} {
   	encode zstd gzip
   	# Caddy proxies WebSocket upgrades (/ws) transparently.
   	reverse_proxy mumweb:8080
   }
   ```
   `.env`:
   ```sh
   MUMWEB_DOMAIN=ses.alanadiniz.com
   MUMBLE_SUPERUSER_PASSWORD=degistir
   ```
3. **Başlatın:**
   ```sh
   docker compose up -d
   docker compose ps          # mumweb "healthy" olmalı
   docker compose logs -f caddy   # ilk açılışta sertifika alınmasını izleyin
   ```
4. `https://ses.alanadiniz.com` adresini açın. Masaüstü Mumble istemcileri aynı sunucuya
   `ses.alanadiniz.com:64738` ile bağlanabilir.

Compose dosyası üç servis çalıştırır: `mumble` (resmî `mumblevoip/mumble-server`), `mumweb` (gateway + arayüz,
root olmayan kullanıcı, `/healthz` sağlık kontrolü) ve `caddy` (otomatik HTTPS, WebSocket'i olduğu gibi aktarır).
Aynı dosyalar bu depoda [`deploy/`](deploy/) altındadır (yorumlu bir `.env.example` ile).

Üç imajın da (`mumweb`, `mumble-server`, `caddy`) `docker-compose.yml` içinde **digest'i sabittir**; bu yüzden
`docker compose pull` siz `.env` içinde `MUMWEB_IMAGE`, `MUMBLE_IMAGE` veya `CADDY_IMAGE`'ı bilerek değiştirmeden hiçbir
şeyi değiştirmez (geçerli varsayılanlar ve yenilerini nereden bulacağınız `deploy/.env.example`'da). İmajlar
`linux/amd64` ve `linux/arm64` için yayınlanır.

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

Mumble sunucusu kullanıcı başına mesaj hızını sınırlar (`messagelimit`=1/sn, `messageburst`=5; kanala katılma,
mute, kanal oluşturma/ACL, sohbet mesajı) ve fazlasını **sessizce düşürür**. Çok hızlı art arda yapılan işlemlerden
biri olmazsa birkaç saniye sonra tekrar deneyin; masaüstü istemci için de aynı sınır geçerlidir.

`SuperUser` hesabı Mumble'da tasarım gereği konuşamaz (yalnızca yönetim içindir). Ana web istemcisinden SuperUser girişi
`MUMWEB_ALLOW_SUPERUSER` ile açılır; production'da kapalıdır. Yönetim için [yönetim panelini](#yönetim-paneli-admin)
(`MUMWEB_ADMIN_PASSWORD`) veya masaüstü istemciyi kullanın.

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
| `MUMWEB_MAX_SESSIONS_PER_IP` | `10` | İstemci IP'si başına eşzamanlı web oturumu sınırı (`0` = kapalı). Aynı NAT arkasındakiler paylaşır; proxy arkasında `MUMWEB_TRUST_PROXY` ayarlayın |
| `MUMWEB_ALLOWED_ORIGINS` | boş | Virgülle ayrılmış izinli `Origin`'ler (ör. `https://ses.alanadi.com`; varsayılan port yazılsa da olur); boşsa yalnızca aynı host |
| `MUMWEB_CONN_RATE` | `10` | IP başına dakikada yeni bağlantı sınırı (`0` = kapalı) |
| `MUMWEB_TRUST_PROXY` | `false` | İstemci IP'sini `X-Forwarded-For`'dan al (ters proxy arkasında `true`) |
| `MUMWEB_ALLOW_SUPERUSER` | `false` | Web'den Mumble `SuperUser` hesabıyla girişe izin ver (kapalıyken bu ad ana istemcide reddedilir; yönetim için yönetim paneli veya masaüstü istemci) |
| `MUMWEB_ACCESS_PASSWORD` | boş | Doluysa her web kullanıcısı giriş ekranında bu şifreyi girmek zorundadır. Mumble sunucu şifresinden ayrıdır ve yalnızca gateway uygular (aşağıya bakın) |
| `MUMWEB_ADMIN_PASSWORD` | boş | Doluysa `/admin` yönetim panelini açar; yönetim oturumları bunu göndermek zorundadır. Boşsa panel kapalıdır (aşağıya bakın) |
| `MUMWEB_RESERVED_SLOTS` | `0` | `MUMWEB_MAX_SESSIONS` içinden kaç slotun `MUMWEB_RESERVED_PASSWORD` sahiplerine (ve yönetim oturumlarına) ayrılacağı. Diğer kullanıcılar kalan slotları paylaşır. Şifre ve ayrılan sayıdan büyük bir sınır gerekir, yoksa kapalıdır |
| `MUMWEB_RESERVED_PASSWORD` | boş | Ayrılmış slotların şifresi; giriş ekranındaki isteğe bağlı alana yazılır |
| `MUMWEB_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

### Web erişim şifresi

Web istemcisini yabancılardan korumak için `MUMWEB_ACCESS_PASSWORD` ayarlayın. Giriş ekranında ek bir "Erişim şifresi"
alanı çıkar ("bu tarayıcıda hatırla" seçeneğiyle); alan yalnızca özellik açıkken görünür.

- Mumble sunucu şifresi **değildir**. Masaüstü Mumble istemcileri doğrudan 64738 portuna bağlanır ve bu şifreyi görmez;
  sunucunun kendisini de korumak istiyorsanız Mumble'ın kendi şifresini kullanın.
- Gateway şifreyi Mumble'a bağlanmadan önce sabit zamanlı karşılaştırır, yanlış denemeden sonra bir saniye bekler ve
  5 yanlış denemeden sonra o IP'yi 10 dakika engeller.
- Diğer her şey gibi HTTPS/WSS üzerinden gider, bu yüzden HTTPS kullanın. Kullanıcı başına kimlik doğrulama değil,
  ortak bir "kapı kodu" olarak düşünün.

### Ayrılmış web slotları

`MUMWEB_MAX_SESSIONS`, Mumble sunucusu ne kadar izin verirse versin web istemcisini aynı anda kaç kişinin kullanacağını
sınırlar (örneğin 50 kişilik sunucuda `MUMWEB_MAX_SESSIONS=10`). Bunlardan bazılarını kendinize ve arkadaşlarınıza ayırmak
için (oyun sunucularındaki yönetici slotu gibi) `MUMWEB_RESERVED_SLOTS=2` ve `MUMWEB_RESERVED_PASSWORD` ayarlayın. 10 web
slotunda herkes 8'ini paylaşır; giriş ekranındaki isteğe bağlı "Ayrılmış slot şifresi" alanına şifreyi yazanlar son 2 slotu
da kullanabilir. Alan yalnızca özellik açıkken görünür.

- 8 slot doluyken gelen normal kullanıcı "Web slotları dolu" mesajını görür ve şifreyle tekrar deneyebilir.
- Yönetim paneli oturumları da ayrıcalıklı sayılır; sunucuyu yönetmek için her zaman girebilirsiniz.
- Yanlış ayrılmış slot şifresi reddedilir (erişim şifresiyle aynı bekleme ve 10 dakikalık kilit, ayrı sayaçla); böylece yazım
  hatası sessizce genel bir slot harcamaz.
- Erişim şifresi gibi ortak bir "kapı kodu"dur: HTTPS kullanın. Masaüstü Mumble istemcileri etkilenmez.

### Yönetim paneli (`/admin`)

`https://alan-adiniz/admin/` adresindeki yönetim panelini açmak için `MUMWEB_ADMIN_PASSWORD` ayarlayın. Ayarlı değilse
panel kapalıdır ve girişi reddedilir. Panel, Mumble protokolünün sunduklarını yönetir:

- **Çevrimiçi:** at, yasakla (yalnızca sertifika), sunucu tarafı sustur/sağır et, kaydet.
- **Kanallar:** oluşturma / yeniden adlandırma / silme ile kanal ağacı, kanal şifresi ve tam bir **ACL ve grup
  düzenleyicisi** (kurallar ve sıraları, izin başına izin ver/reddet, devralma bayrakları, grup üyeleri).
- **Yasaklar:** yasak listesi. Her kaydı silebilirsiniz (masaüstü istemciden yapılan IP yasakları dahil) ve sertifika
  yasağı ekleyebilirsiniz.
- **Kayıtlı kullanıcılar:** listele, yeniden adlandır, sil.

Sunucu ayarları (karşılama metni, bant genişliği, kullanıcı sınırı) Mumble protokolünde yoktur; sunucunun yapılandırmasından
değiştirin.

- **Ayrı bir hesapla girin.** Panel önce yönetici şifresini, sonra bir Mumble hesabını ister; genellikle `SuperUser` ve
  Mumble yönetici şifresi. Ana istemci için `MUMWEB_ALLOW_SUPERUSER` `false` olsa da SuperUser burada çalışır; yani ana
  girişi kapalı tutabilirsiniz. Gerekli izinlere sahip kayıtlı kullanıcılar da (Ban / Register / Write izinli bir grup)
  çalışır.
- **Aynı tarayıcıda aynı anda sesli sohbet ve yönetim.** Panel kendi sertifikasını (kimliğini) tutar ve ses taşımaz; bu
  yüzden `/admin` açıkken başka bir sekmede sesli istemci çalışabilir. İki farklı hesap kullanın: aynı kayıtlı hesap iki
  kez bağlanırsa Mumble eski oturumu düşürür.
- Yönetici şifresi bu sayfada web erişim şifresinin yerini alır. Gateway onu Mumble'a bağlanmadan önce kontrol eder ve
  kendi kaba kuvvet kilidi vardır (5 yanlış denemede IP 10 dakika engellenir; erişim şifresinin kilidinden ayrıdır).
  Her işlem için izinleri yine Mumble sunucusu uygular.
- Durumu değiştiren her yönetim işlemi loglanır (işlem, kimlikler, sayılar; asla şifre, sohbet veya ACL grup adı, çünkü
  `#ad` biçimli grup bir kanal şifresidir). Yazma işlemleri Mumble'ın mesaj hız sınırının altında kalacak şekilde
  yavaşlatılır.
- Güvenlik incelemesi, tehdit modeli ve `/admin`'i kendi IP'nize kısıtlayan Caddy örneği: [docs/tr/SECURITY.md](docs/tr/SECURITY.md).
- Sunucuda 1-2 günlük test dağıtımı (gevşek limitler, adım adım kılavuz): [docs/tr/TEST-DEPLOYMENT.md](docs/tr/TEST-DEPLOYMENT.md).
- Production'da HTTPS gerekir (şifre WebSocket üzerinden gider). Değeri uzun ve rastgele seçin. Sayfanın hiç
  erişilebilir olmasını istemiyorsanız ters proxy'nizde `/admin*` yolunu engelleyin.

Mumble sunucusunun parmak izini `mumprobe` yazdırır (`make gateway` sonrası):
`./bin/mumprobe -addr <sunucu>:64738 -exit` → `server certificate SHA-256: AB:CD:…`. Bu değer
`MUMWEB_TLS_FINGERPRINT`'e olduğu gibi yazılabilir. Not: sunucu sertifikası yenilenirse parmak izi değişir.

## Güncelleme

```sh
docker compose pull          # en yeni mumweb (ve Mumble) imajlarını indir
docker compose up -d         # yalnızca değişenleri yeniden oluştur
docker image prune -f        # eski imajları temizle (isteğe bağlı)
```

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

## Kaynaktan derleme

Çoğu kişi yukarıdaki Docker Hub imajını kullanmalı. Kendiniz derlemek, kodu değiştirmek veya testleri çalıştırmak için
**[docs/tr/BUILDING.md](docs/tr/BUILDING.md)** sayfasına bakın.

## Dokümantasyon

Ana sürüm İngilizcedir: [README.md](README.md), [docs/](docs/). Türkçe belgeler `docs/tr/` altındadır.
Derleme, test ve imaj yayınlama: [docs/tr/BUILDING.md](docs/tr/BUILDING.md).

## Lisans

[MIT](LICENSE)
