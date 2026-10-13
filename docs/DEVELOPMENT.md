# Yerel test ortamı (Windows için adım adım)

Bu rehber mumweb'i kendi bilgisayarınızda **gerçek HTTPS ile** çalıştırmayı anlatır. Sonuçta tarayıcıdan
`https://mumweb.test` adresini açıp sesli sohbeti deneyebilir, aynı sunucuya masaüstü Mumble istemcisiyle de
bağlanabilirsiniz. Windows için yazıldı; macOS/Linux farkları en sonda.

```
 Tarayıcı ──► https://mumweb.test ──► Caddy (mkcert sertifikası) ──► mumweb ──► Mumble sunucusu
                                                                                    ▲
 Masaüstü Mumble istemcisi ──────────────► mumweb.test:64738 ───────────────────────┘
```

`deploy/local/compose.yaml` üç container çalıştırır:

| Servis | Görevi |
|---|---|
| `certs` | Tek seferlik: mkcert dosyalarını Mumble'ın okuyabileceği izinlerle bir volume'e kopyalar, sonra kapanır. |
| `mumble` | Resmî Mumble sunucusu (`mumblevoip/mumble-server`), mkcert sertifikasıyla. 64738 portu masaüstü istemciler için açık. |
| `mumweb` | Bu depodan derlenen gateway + web arayüzü. |
| `caddy` | `https://mumweb.test`'i mkcert ile ürettiğiniz sertifikayla sunar, isteği mumweb'e aktarır. |

> **Neden HTTPS?** Tarayıcılar mikrofon erişimine ve WebCodecs ses kodlamasına yalnızca güvenli bağlamda
> (HTTPS veya `localhost`) izin verir. mkcert, bilgisayarınızın güvendiği yerel bir sertifika otoritesi
> oluşturur; böylece tarayıcı uyarı vermeden HTTPS kullanır.

## Hızlı başlangıç

Araçlar kuruluysa beş adımda çalışır. Ayrıntılar aşağıda.

```powershell
# 1) Bir kez: mkcert'in yerel sertifika otoritesini kur
mkcert -install

# 2) Sertifikayı üret
cd mumweb\deploy\local
mkcert -cert-file certs\cert.pem -key-file certs\key.pem mumweb.test localhost 127.0.0.1 ::1

# 3) hosts dosyasına ekle (Yönetici PowerShell'de)
Add-Content -Path "$env:SystemRoot\System32\drivers\etc\hosts" -Value "`n127.0.0.1 mumweb.test`n::1 mumweb.test"

# 4) Başlat
docker compose up -d --build

# 5) Aç: https://mumweb.test
```

---

## 1. Gereksinimler

| Araç | Neden | Kurulum |
|---|---|---|
| **Docker Desktop** (WSL 2 altyapısı) | Container'ları çalıştırır | <https://www.docker.com/products/docker-desktop/> veya `winget install Docker.DockerDesktop` |
| **Git** | Kodu almak için | `winget install Git.Git` |
| **mkcert** | Yerel, güvenilir HTTPS sertifikası | `winget install FiloSottile.mkcert` (alternatif: `choco install mkcert` / `scoop install mkcert`) |
| Mumble masaüstü istemcisi (isteğe bağlı) | Web ↔ masaüstü testleri | <https://www.mumble.info/downloads/> |

Go, Node veya başka bir derleme aracı **gerekmez**; her şey Docker içinde derlenir.

`winget` ile kurulumdan sonra yeni bir PowerShell penceresi açın ki komutlar `PATH`'e girsin. Docker Desktop'ın
çalıştığını kontrol edin:

```powershell
docker version
docker compose version
mkcert -version
```

## 2. Kodu alın

```powershell
git clone <repo-adresi> mumweb
cd mumweb
```

Depodaki `.gitattributes`, container içinde okunan dosyaların (Caddyfile, compose, Dockerfile) Windows'ta da LF satır
sonuyla gelmesini sağlar; ek bir Git ayarı gerekmez.

## 3. mkcert ile sertifika

### 3.1 Yerel sertifika otoritesini kurun (bir kez)

```powershell
mkcert -install
```

Windows bir güvenlik uyarısı gösterir ("Bu sertifikayı yüklemek istiyor musunuz?"); **Evet** deyin. Bu adım
mkcert'in kök sertifikasını Windows sertifika deposuna ekler. Chrome ve Edge bu depoyu kullanır.

> **Firefox:** Güncel Firefox sürümleri Windows deposundaki kök sertifikalara güvenir. Yine de sertifika uyarısı
> görürseniz `about:config` → `security.enterprise_roots.enabled` = `true` yapıp Firefox'u yeniden başlatın.

### 3.2 `mumweb.test` için sertifika üretin

```powershell
cd deploy\local
mkcert -cert-file certs\cert.pem -key-file certs\key.pem mumweb.test localhost 127.0.0.1 ::1
```

Dosya adları önemli: Caddy `certs\cert.pem` ve `certs\key.pem` dosyalarını okur. `certs` klasörünün içeriği git'e
eklenmez (özel anahtar sizde kalır).

> Farklı bir alan adı kullanmak isterseniz (ör. `ses.test`) sertifikayı o adla üretin, hosts dosyasına onu yazın
> ve `.env` içinde `MUMWEB_DOMAIN=ses.test` ayarlayın (bkz. [Ayarlar](#7-ayarlar-env)).
> `.local` uzantılı adlardan kaçının; Windows bunları ağda (mDNS) arar ve gecikme olur. `.test` bu iş için ayrılmıştır.

## 4. hosts dosyası

`mumweb.test` adının bilgisayarınızı göstermesi gerekir.

**Yöntem A — Not Defteri:** Başlat menüsünde *Not Defteri*'ne sağ tıklayın → **Yönetici olarak çalıştır** →
`C:\Windows\System32\drivers\etc\hosts` dosyasını açın (dosya türü: *Tüm Dosyalar*) ve en alta ekleyin:

```
127.0.0.1 mumweb.test
::1 mumweb.test
```

**Yöntem B — PowerShell (Yönetici olarak):**

```powershell
Add-Content -Path "$env:SystemRoot\System32\drivers\etc\hosts" -Value "`n127.0.0.1 mumweb.test`n::1 mumweb.test"
```

Ardından DNS önbelleğini temizleyip kontrol edin:

```powershell
ipconfig /flushdns
ping mumweb.test     # "127.0.0.1" veya "::1" yanıt vermeli
```

## 5. Başlatın

```powershell
cd deploy\local        # depo kökünden
docker compose up -d --build
```

İlk çalıştırmada imajlar indirilir ve web arayüzü ile gateway derlenir; birkaç dakika sürebilir. Sonraki
başlatmalar saniyeler sürer.

Durumu kontrol edin:

```powershell
docker compose ps
```

`mumweb` satırı `healthy` olmalı, `caddy` ve `mumble` `Up` görünmeli. `certs` servisinin
`Exited (0)` görünmesi normaldir (işini yapıp kapanır; `docker compose ps -a` ile görünür).

## 6. Kullanın

### 6.1 Tarayıcıdan

1. Chrome, Edge veya Firefox'ta **<https://mumweb.test>** adresini açın. Adres çubuğunda kilit simgesi görünmeli,
   uyarı çıkmamalı.
2. Bir takma ad yazıp **Bağlan**'a tıklayın. Tarayıcı mikrofon izni ister; izin verin.
3. Kanal ağacı, sohbet ve altta kontrol çubuğu gelir. Bir kullanıcıya **sağ tıklayarak** onun sesini sadece kendiniz
   için kısabilir/susturabilir, bir kanala sağ tıklayıp (veya çift tıklayıp) o kanala geçebilirsiniz. Konuşma modu varsayılan olarak **Ses algılama**'dır;
   konuşurken seviye çubuğu sarı çizgiyi geçmeli. **Bas-konuş** modunda varsayılan tuş `V`'dir (Ayarlar'dan değişir).

**Tek bilgisayarda iki kullanıcı:** Normal pencerede bir kullanıcıyla, **gizli/InPrivate pencerede** başka bir
takma adla girin. Gizli pencere ayrı bir sertifika (kimlik) kullanır. Yankı ve geri besleme olmaması için
**kulaklık takın** veya birini sessize alın.

### 6.2 SuperUser ile yönetim (kanal oluşturma)

Yerel ortamda Mumble'ın yönetici hesabı **SuperUser** web'den de kullanılabilir:

1. Giriş ekranında takma ad: `SuperUser`, **Sunucu şifresi**: `superuser` (`.env`'deki `MUMBLE_SUPERUSER_PASSWORD`).
2. Bir kanala **sağ tıklayın → Alt kanal oluştur…** → adı yazıp **Oluştur**. "Geçici kanal" seçilirse kanal
   herkes çıkınca silinir.
3. Kanala geçmek için çift tıklayın veya **sağ tık → Kanala katıl**.

4. Kullanıcılara **sağ tıklayarak** masaüstü Mumble'daki yönetim menüsünü kullanabilirsiniz: kov, yasakla
   (yalnızca sertifika), sustur/sağır et, imtiyazlı konuşmacı, kendi kanalına taşı, kaydet, açıklamayı sıfırla,
   bilgi, özel mesaj. Rumuz, arkadaş ve "mesajları görmezden gel" yalnızca sizin tarayıcınızda saklanır.

SuperUser Mumble'da **konuşamaz** (tasarım gereği); ses testi için normal bir takma adla, yönetim için SuperUser ile
(ör. gizli pencerede) girin. Normal (kayıtsız) kullanıcılar Mumble'ın varsayılan izinleriyle kanal oluşturamaz; denerlerse sohbette
"Bu işlem için yetkiniz yok." görürler. Bu davranış sunucunun ACL ayarlarından gelir.

> Web'den SuperUser girişi `MUMWEB_ALLOW_SUPERUSER` ile açılıp kapanır. Yerel ortamda **açık**, production
> compose'ta **kapalı** gelir; web giriş sayfası adresi bilen herkese açık olduğundan gerçek sunucuda yönetim için
> masaüstü istemci önerilir.

### 6.3 Masaüstü Mumble istemcisiyle

1. Mumble'da **Sunucu → Bağlan → Sunucu ekle**:
   - Adres: `mumweb.test`
   - Port: `64738`
   - Kullanıcı adı: istediğiniz bir ad
2. Mumble sunucusu da `certs\` klasöründeki mkcert sertifikasını kullanır; mkcert'in kök sertifikası Windows'a
   kurulu olduğu için istemci **sertifika sormadan** bağlanır. (`localhost` veya IP ile bağlanırsanız ad
   uyuşmadığı için sorabilir; `mumweb.test` kullanın.)
3. Web'deki kullanıcıları görürsünüz; konuşma, sohbet ve kanal değiştirme iki yönde çalışmalı.

Masaüstünden yönetim için aynı şekilde `SuperUser` / `superuser` ile bağlanıp kanala sağ tık → **Ekle** diyebilirsiniz.

Denenecek senaryoların listesi için README'deki
[Manuel kabul testleri](../README.md#manuel-kabul-testleri) bölümüne bakın.

## 7. Ayarlar (.env)

Varsayılanlarla `.env` dosyası gerekmez. Değiştirmek isterseniz:

```powershell
copy .env.example .env
notepad .env
docker compose up -d       # değişiklikleri uygular
```

| Değişken | Varsayılan | Açıklama |
|---|---|---|
| `MUMWEB_DOMAIN` | `mumweb.test` | hosts dosyasına ve sertifikaya yazdığınız ad |
| `HTTPS_PORT` | `443` | Tarayıcının bağlanacağı port |
| `HTTP_PORT` | `80` | HTTP → HTTPS yönlendirmesi |
| `MUMBLE_PORT` | `64738` | Masaüstü Mumble istemcisinin bağlanacağı port |
| `MUMBLE_SUPERUSER_PASSWORD` | `superuser` | Mumble SuperUser şifresi (ilk çalıştırmada ayarlanır) |
| `MUMWEB_ALLOW_SUPERUSER` | `true` | Web'den `SuperUser` girişine izin ver |
| `MUMWEB_LOG_LEVEL` | `debug` | Gateway log seviyesi |

### Port çakışması (80/443 dolu)

IIS, Skype, VMware veya başka bir proxy 80/443'ü kullanıyorsa Caddy başlamaz ("port is already allocated").
Hangi programın kullandığını görmek için:

```powershell
netstat -ano | findstr ":443"
Get-Process -Id <PID>          # son sütundaki numara
```

`netstat` boş döndüğü hâlde port kullanılamıyorsa Windows (Hyper-V/WSL) o aralığı ayırmış olabilir:

```powershell
netsh interface ipv4 show excludedportrange protocol=tcp
```

Programı kapatabilir veya `.env` içinde listede olmayan başka bir port seçebilirsiniz:

```
HTTPS_PORT=8443
HTTP_PORT=8080
```

Bu durumda adres **<https://mumweb.test:8443>** olur (gateway'in izin verdiği adres de buna göre otomatik ayarlanır).
Not: HTTP portundan gelen yönlendirme portu bilmez; doğrudan `https://…:8443` adresini kullanın.

## 8. Kodu değiştirdikten sonra

Arayüz ve gateway imajın içinde derlenir. Değişiklikten sonra yalnızca mumweb'i yeniden derleyin:

```powershell
docker compose up -d --build mumweb
```

Tarayıcıda sayfayı yenileyin. Uygulama ana ekrana eklenebilir olduğu için bir service worker statik dosyaları
önbelleğe alır. Sayfanın kendisi her zaman ağdan yüklendiği için normal yenileme yeterlidir. Eski arayüz görünürse
**Ctrl+F5** yapın; o da olmazsa geliştirici araçlarında *Application → Service workers → Unregister* seçin.

Sık arayüz değişikliği yapıyorsanız ve Node kuruluysa, `web` klasöründe `npm run dev` ile anlık yenilemeli
(Vite) geliştirme de mümkündür. Bunun için gateway'in `localhost:8080`'de çalışması gerekir (README → Geliştirme,
`make dev`).

## 9. Loglar, durdurma, sıfırlama

```powershell
docker compose logs -f mumweb        # gateway (JSON) — bağlantılar, hatalar
docker compose logs -f mumble        # Mumble sunucusu
docker compose logs -f caddy         # HTTPS / proxy

docker compose stop                  # durdur (veriler kalır)
docker compose start                 # tekrar başlat
docker compose down                  # container'ları kaldır (Mumble kanalları/kayıtları volume'de kalır)
docker compose down -v               # her şeyi sıfırla: kanallar, kayıtlı kullanıcılar, SuperUser şifresi
```

## 10. Aynı ağdaki başka bir cihazdan test

Telefondan veya başka bir bilgisayardan denemek için o cihazın da (a) `mumweb.test`'i sizin bilgisayarınıza
çözmesi ve (b) mkcert sertifikasına güvenmesi gerekir.

1. **Bilgisayarınızın yerel IP'si:** `ipconfig` → *IPv4 Address* (ör. `192.168.1.20`).
2. **Sertifikayı bu IP için de üretin** (isteğe bağlı ama önerilir):
   ```powershell
   mkcert -cert-file certs\cert.pem -key-file certs\key.pem mumweb.test localhost 127.0.0.1 ::1 192.168.1.20
   docker compose up -d --force-recreate certs mumble caddy
   ```
3. **Güvenlik duvarı:** Yönetici PowerShell'de:
   ```powershell
   New-NetFirewallRule -DisplayName "mumweb local" -Direction Inbound -Protocol TCP -LocalPort 80,443,64738 -Action Allow
   ```
4. **Kök sertifikayı diğer cihaza kurun:** Kök sertifika `mkcert -CAROOT` komutunun gösterdiği klasördeki
   `rootCA.pem` dosyasıdır (**`rootCA-key.pem`'i asla paylaşmayın**).
   - Başka bir Windows PC: dosyayı `rootCA.crt` olarak kopyalayıp çift tıklayın → *Sertifika yükle* →
     *Yerel Makine* → *Güvenilen Kök Sertifika Yetkilileri*.
   - Android: dosyayı telefona atın → *Ayarlar → Güvenlik → Şifreleme ve kimlik bilgileri → Sertifika yükle →
     CA sertifikası*. (Menü adları üreticiye göre değişir.)
5. **Ad çözümleme:**
   - Başka bir PC: onun hosts dosyasına `192.168.1.20 mumweb.test` ekleyin.
   - Android'de hosts dosyası düzenlenemez. Ağınızda bir yerel DNS varsa (ör. AdGuard Home / Pi-hole
     "DNS rewrite", bazı modemlerin yerel DNS kaydı) `mumweb.test → 192.168.1.20` kaydı ekleyin.
6. Diğer cihazda `https://mumweb.test` adresini açın.

> iOS/Safari henüz desteklenmiyor (Safari'de WebCodecs Opus yok).

## 11. Sorun giderme

| Belirti | Çözüm |
|---|---|
| `docker compose up` "service certs didn't complete successfully" ile duruyor / `caddy` logunda `open /certs/cert.pem: no such file` | Sertifika üretilmemiş veya adı farklı: [3.2](#32-mumwebtest-için-sertifika-üretin)'deki komutu `deploy\local` içinde çalıştırın, sonra `docker compose up -d` |
| Masaüstü Mumble yine de sertifika soruyor | `mumweb.test` adıyla bağlanın; sertifikayı sonradan yenilediyseniz `docker compose up -d --force-recreate certs mumble` |
| `NET::ERR_CERT_AUTHORITY_INVALID` / "Bağlantınız gizli değil" | `mkcert -install` çalıştırılmamış ya da tarayıcı açıkken kurulmuş: tarayıcıyı tamamen kapatıp açın. Firefox için [3.1](#31-yerel-sertifika-otoritesini-kurun-bir-kez)'deki ayar |
| `NET::ERR_CERT_COMMON_NAME_INVALID` | Sertifika başka bir ad için üretilmiş; açtığınız adı (`mumweb.test`) içerecek şekilde yeniden üretin |
| "Bu siteye ulaşılamıyor" / `DNS_PROBE_FINISHED_NXDOMAIN` | hosts satırı eksik veya yanlış; `ping mumweb.test` ile kontrol edin, `ipconfig /flushdns`. Tarayıcıda VPN veya kurumsal proxy varsa kapatın |
| `port is already allocated` | 80/443/64738 dolu: [Port çakışması](#port-çakışması-80443-dolu) |
| Sayfa açılıyor ama hep "Yeniden bağlanılıyor"; `docker compose logs mumweb`'de `ws origin rejected` | Adres çubuğundaki adres ile `.env`'deki `MUMWEB_DOMAIN`/`HTTPS_PORT` uyuşmuyor. `localhost` yerine `https://mumweb.test` kullanın |
| "Mumble sunucusuna ulaşılamıyor" | `docker compose ps` ile `mumble`'ın çalıştığını kontrol edin; `docker compose logs mumble` |
| "Mikrofon kullanılamıyor" / "Mikrofon açılamadı" | Uyarının altındaki parantezde gerçek hata yazar. `NotReadableError`/`AbortError`: mikrofonu başka bir uygulama kullanıyor (Teams, Discord, OBS…) veya Windows engelliyor → *Ayarlar → Gizlilik ve güvenlik → Mikrofon* altında hem "Mikrofon erişimi" hem "Masaüstü uygulamalarının mikrofonunuza erişmesine izin verin" açık olmalı. `NotFoundError` ("Requested device not found", izin penceresi hiç çıkmaz): tarayıcı hiç mikrofon görmüyor; uyarıdaki "gördüğü mikrofon sayısı" 0 olur → *Ayarlar → Sistem → Ses → Giriş*'te cihaz var mı, gizlilikteki iki anahtar açık mı, Uzak Masaüstü/VM'de mikrofon yönlendirmesi açık mı kontrol edin, tarayıcıyı tamamen kapatıp açın. Düzelttikten sonra **Tekrar dene**'ye basın |
| Mikrofon izni istenmiyor / "izin verilmedi" | Adres çubuğundaki mikrofon/kilit simgesinden siteye izin verin (Firefox: kilit → *Bağlantı güvenli* yanındaki izinler), sonra **Tekrar dene** |
| Karşı taraf kendi sesini yankı olarak duyuyor | Hoparlör sesi mikrofona giriyor: kulaklık kullanın veya Bas-konuş moduna geçin |
| Sesler kesik kesik | Docker Desktop'a yeterli CPU/RAM verildiğini kontrol edin (*Settings → Resources*); arka planda ağır işler varsa kapatın; mümkünse iki farklı cihazla deneyin |
| Caddy logunda Caddyfile ayrıştırma hatası veya derlemede `\r` hatası | Dosyalar CRLF ile çekilmiş (`.gitattributes` eklenmeden önceki bir klon). En kolayı depoyu yeniden klonlamak. Yerinde düzeltmek için `git rm --cached -r . ; git reset --hard` (**commit edilmemiş değişiklikler silinir**) |
| Değişikliklerim görünmüyor | `docker compose up -d --build mumweb`, ardından tarayıcıda Ctrl+F5 ([8](#8-kodu-değiştirdikten-sonra)) |

## 12. macOS / Linux farkları

Adımlar aynı. Değişenler:

- mkcert kurulumu: macOS `brew install mkcert nss` · Debian/Ubuntu `sudo apt install mkcert libnss3-tools`
- hosts dosyası: `/etc/hosts`, örneğin
  `echo "127.0.0.1 mumweb.test" | sudo tee -a /etc/hosts`
- Komutlarda `\` yerine `/`: `mkcert -cert-file certs/cert.pem -key-file certs/key.pem mumweb.test localhost 127.0.0.1 ::1`
- Linux'ta 1024 altı portlar Docker ile sorunsuz yayınlanır; yalnızca başka bir web sunucusunun kullanmadığından emin olun.

## Diğer geliştirme ortamları

| Dosya | Ne zaman |
|---|---|
| `deploy/local/compose.yaml` | **Bu rehber:** HTTPS ile uçtan uca deneme, masaüstü istemciyle karşılaştırma |
| `deploy/docker-compose.dev.yml` (`make dev`) | Gateway geliştirme ve Go entegrasyon testleri; HTTPS yok, `http://localhost:8080` (localhost güvenli bağlam sayılır) |
| `deploy/docker-compose.yml` | Gerçek sunucuya kurulum (Let's Encrypt), bkz. README |
