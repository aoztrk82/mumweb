# Güvenlik incelemesi ve duruş

[English](../SECURITY.md) | **Türkçe**

Hedef: hiçbir girdi yolu container dışına veya sunucu sistemine erişim sağlamamalı ve bir web kullanıcısı bir
başkasına zarar verememeli. Bu sayfa tehdit modelini, uygulanan önlemleri, bulunan ve karara bağlanan şeyleri, kabul
edilen riskleri ve kontrollerin nasıl tekrarlanacağını kaydeder. İlk inceleme: 2026-10-14 (0.2.0 sürümü ve o günkü
sertleştirmeler). Açık bildirme: [SECURITY.md](../../SECURITY.md) (İngilizce).

## 1. Tehdit modeli

| Aktör | Yapabilir | Yapamamalı |
|---|---|---|
| Anonim internet kullanıcısı | sayfayı açar, WebSocket açar, hello gönderir | yapılandırılandan başka bir Mumble sunucusuna ulaşmak; yavaşlatılmadan ve kilitlenmeden şifre denemek; gateway'i tüketmek |
| Kötü niyetli web kullanıcısı (girişli) | her JSON / ikili çerçeveyi gönderir, komut yağdırır, ad seçer, HTML sohbet yazar | başka web kullanıcısı gibi davranmak veya onu atmak, başkasının adını almak, yetkisi olmayan ses/yönetim komutu göndermek, başkalarının tarayıcısında betik çalıştırmak |
| Kötü niyetli Mumble kullanıcısı | sohbet, yorum, kanal açıklaması (HTML), çok büyük veya garip değerler gönderir | web kullanıcısının tarayıcısında betik çalıştırmak, gateway'i çökertmek |
| Kötü niyetli veya ele geçirilmiş Mumble sunucusu | gateway'e her protokol mesajını gönderir | gateway'de kod çalıştırmak (sunucu yapılandırılmış, güvenilen bir uç; ama ayrıştırma çökmemeli) |
| Çalınan tarayıcı verisi | IndexedDB ve localStorage'ı okur | (önlenemez, bkz. 4.7: kimlik anahtarı açıkta) |
| Yönetici şifresini bilen biri | `/admin`'i kullanır | seçtiği Mumble hesabının yapabildiğinin ötesine geçmek |

Güven sınırları: tarayıcı ⇄ (TLS, Caddy) ⇄ gateway ⇄ (TLS) ⇄ Mumble sunucusu. Gateway diskte sır tutmaz ve sertifika
saklamaz; bir kullanıcının oturumu açıkken özel anahtarını bellekte görür (bkz. 4.7).

## 2. Uygulanan önlemler

| Alan | Önlem |
|---|---|
| Hedef | tek ve sabit `MUMWEB_MUMBLE_ADDR`; tarayıcı adres seçemez (SSRF yok) |
| WebSocket | Origin izin listesi (`MUMWEB_ALLOWED_ORIGINS`), 16 KiB JSON sınırı (yönetim oturumlarında 512 KiB), 4 KiB ses çerçevesi, 10 sn içinde hello, 60 sn boşta zaman aşımı, 10 sn yazma zaman aşımı |
| Hız ve sınırlar | IP başına yeni bağlantı (`MUMWEB_CONN_RATE`), **IP başına eşzamanlı oturum** (`MUMWEB_MAX_SESSIONS_PER_IP`, varsayılan 10), toplam oturum (`MUMWEB_MAX_SESSIONS`), ses 100 paket/sn (burst 150), **kontrol mesajları 50/sn (burst 200), art arda 500 düşen mesaj oturumu bitirir**, yönetim komutları 0,8/sn'ye yavaşlatılır |
| HTTP sunucusu | başlık zaman aşımı 10 sn, boşta zaman aşımı 120 sn, başlık boyutu sınırı 16 KiB, root olmayan kullanıcı |
| Şifreler | erişim ve yönetici şifresi özetlenip sabit zamanlı karşılaştırılır; yanlış denemede 1 sn gecikme; 5 yanlış deneme IP'yi 10 dakika kilitler; erişim ve yönetici için ayrı sayaç; asla loglanmaz |
| Adlar | başka sertifikalı bir oturumun kullandığı ad reddedilir (`UsernameInUse`); kimse adı yeniden kullanarak başka web kullanıcısını atamaz |
| SuperUser | ana girişte varsayılan olarak reddedilir; yalnızca yönetici şifresini geçen yönetim oturumlarında serbest |
| Yönetim paneli | `MUMWEB_ADMIN_PASSWORD` olmadan kapalı; yönetici olmayan oturumlarda yönetim komutları reddedilir; panelden yeni yasaklar yalnızca sertifika yasağı olabilir; SuperUser kaydı değiştirilemez; boyutlar ve izin maskeleri sınırlı; durumu değiştiren her işlem denetlenir (kimlikler, asla sırlar) |
| Tarayıcı | **Content-Security-Policy** (betikler yalnızca gateway'den: satır içi betik yok, eval yok; çerçeveleme yok; `<object>`, `<base>` yok), `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, `Permissions-Policy` (mikrofon yalnızca bu origin için), COOP `same-origin`; **HSTS** Caddy'den |
| Sohbet ve yorumlar | Mumble'dan gelen tüm HTML etiket/öznitelik izin listeli DOMPurify'dan geçer, yalnızca `http(s)`/`mailto` bağlantıları (`rel=noopener noreferrer nofollow`), yalnızca satır içi `data:image` resimleri; yazılan metin kaçışlanır; 21 XSS yükü birim testlidir |
| Container | `read_only`, `cap_drop: ALL`, `no-new-privileges`, 256 MiB, 200 işlem, 1 CPU, uid 100; gateway yalnızca Mumble ve Caddy ile bir `internal` ağda durur, yani **internete çıkış yolu yok** (denendi: container'dan `wget` başarısız) |
| Mumble | Ice yönetim arayüzü container içinde 127.0.0.1'de dinler ve yayınlanmaz; autoban paylaşılan gateway IP'si için ayarlı |

## 3. İnceleme kaydı (2026-10-14)

| Kontrol | Sonuç |
|---|---|
| `govulncheck ./...` (Go 1.27.2) | zafiyet yok |
| `npm audit` (`--omit=dev` dahil) | 0 zafiyet |
| `aoztrk82/mumweb:0.2.0` üzerinde Trivy (HIGH, CRITICAL) | Alpine katmanında 0, Go ikilisinde 0 |
| `staticcheck ./...` | bir testte 1 not (`TestIPRateLimit` `allow`'u bilerek üç kez çağırır): yok sayıldı |
| `gosec` | 3 bulgu, üçü de kabul edildi: `certHash` içinde SHA-1 (Mumble sertifika özetlerini SHA-1 olarak tanımlar ve kullanıcılar onunla anahtarlanır; güvenlik sınırı değil), aynı import'un yeniden işaretlenmesi (G505) ve `mumprobe` hata ayıklama CLI'ındaki `os.ReadFile(path)` (yol operatörün kendi argümanı) |
| Fuzz (`testing.F`, her biri 25 sn) | `FuzzParseAudioFrame`, `FuzzClientMessage` (JSON ve yönetim doğrulayıcıları), `FuzzReadFrame` (Mumble çerçeveleme ve protobuf çözme), `FuzzDecodeAudio`, `FuzzStateMessages` (durum birleştirme ve ACL/yasak/kullanıcı listesi dönüşümleri): çökme yok. Büyük sürümlerden önce daha uzun koşular yapılmalı: `go test ./internal/mumble -run '^$' -fuzz FuzzReadFrame -fuzztime 10m` |
| XSS | `sanitizeHtml` için 21 yük testi ve gerçek tarayıcıda kontrol: yapıştırılan `<img onerror>` metin olarak görünür; CSP sesli istemcide ve yönetim panelinde ihlal üretmedi |
| Log denetimi | her `slog` çağrısı okundu: hiçbir şifre, token, anahtar, sohbet metni veya ACL grup adı loglanmıyor. Loglanan saldırgan kaynaklı metinler (Origin başlığı, bilinmeyen mesaj tipi) log işleyicisi tarafından JSON kaçışlanır. Kullanıcı adları loglanır (operasyonel veri, sır değil) |

## 4. Bulgular ve kararlar

1. **Güvenlik başlıkları yoktu (düzeltildi).** Gateway hiç göndermiyordu. Yukarıdaki CSP ve diğerleri (gateway) ile HSTS
   (Caddy) eklendi. HSTS neden gateway'de değil: geliştirmede düz HTTP üzerinden sunulabilir.
2. **Tek oturumdan sınırsız kontrol mesajı (düzeltildi).** Ses hız sınırlıydı, JSON komutlar değildi; bir oturum 16 KiB'lık
   mesajlarla CPU yakabilirdi. Kontrol kovası eklendi (cömert: giriş her kanalın izinlerini sorar) ve sürekli seldan sonra
   oturum sonlandırılır.
3. **Tek IP tüm oturum yuvalarını alabilirdi (düzeltildi).** IP başına hız (dakikada 10 yeni bağlantı) yine de
   `MUMWEB_MAX_SESSIONS`'ı beş dakikada doldurmaya izin veriyordu. `MUMWEB_MAX_SESSIONS_PER_IP` (varsayılan 10) eklendi.
   Aynı NAT arkasındakiler sınırı paylaşır; büyük ofisler için artırın. Proxy arkasında `MUMWEB_TRUST_PROXY=true`
   ayarlanmalı, yoksa herkes proxy'nin adresi gibi görünür.
4. **Gateway'in dışarı çıkışı ve yazılabilir dosya sistemi vardı (compose dosyalarında düzeltildi).** 2. bölümdeki gibi
   sertleştirildi. Mumble'a yalnızca `no-new-privileges` ve işlem sınırı verildi: `cap_drop: ALL` giriş betiğini bozuyor
   (kendi kullanıcısına geçmeden önce capability ayarlıyor).
5. **Slowloris.** `ReadHeaderTimeout` vardı; `IdleTimeout` ve `MaxHeaderBytes` eklendi. WebSocket okumalarında 60 sn boşta
   zaman aşımı, hello'da 10 sn zaman aşımı var. HTTP sunucusunda bilerek `WriteTimeout` yok: uzun ömürlü WebSocket'i de keserdi.
6. **Yavaş WebSocket okuyucuları.** Her yazmanın 10 sn zaman aşımı var; yavaş tarayıcı kendi oturumunu bitirir. Mumble
   olayları oturum başına tamponlanır (256); dolu tampon yalnızca o Mumble bağlantısını geriye iter.
7. **Özel anahtar XSS'e ve gateway'e açık.** Tarayıcı Mumble kimlik anahtarını IndexedDB'de tutar ve her hello ile gönderir;
   çünkü TLS istemci sertifikasını gateway kurar ve hiçbir şey saklamaz. Bu tasarımla çıkarılamaz (non-extractable)
   WebCrypto anahtarı **mümkün değil**. Azaltmalar: sıkı CSP (satır içi betik yok), DOMPurify, XSS testleri ve anahtarın
   yalnızca o tek Mumble kimliğini vermesi. localStorage'daki kayıtlı kanal şifrelerinin değeri düşük (yalnızca kanal açar).
   Gerçek çözüm TLS istemcisini tarayıcıya taşımak olurdu; Mumble buna izin vermez. **Kabul edildi.**
8. **Ele geçirilmiş gateway = bağlı kullanıcıların tüm anahtarları ve bağlıyken yazılan şifreler.** Anlatıldığı gibi
   çalıştırın (iç ağ, salt okunur, çıkış yok); sunucuyu güncel tutun.
9. **Mumble mesaj hız sınırı.** Sunucu fazlasını sessizce düşürür (sorgular dahil). Zafiyet değil; yönetim panelinde
   yavaşlatma ve yeniden okuma ile ele alındı.
10. **Masaüstünden yapılan IP yasakları tüm web kullanıcılarını vurur.** Bilinen (TODO C). Web yasağı yalnızca sertifikadır
    ve yönetim paneli IP kaydını silebilir.
11. **`X-Forwarded-For`.** `MUMWEB_TRUST_PROXY=true` iken en sağdaki kayda güvenilir. Yalnızca gateway'e sadece
    proxy'niz üzerinden ulaşılıyorsa açın (compose dosyaları portunu yayınlamaz).
12. **`/admin` adresi bilen herkese açık.** Kilitli yönetici şifresiyle korunur. Daha fazlası için Caddy'de kısıtlayın
    (5. bölüm) veya paneli kullanmıyorsanız `MUMWEB_ADMIN_PASSWORD` ayarlamayın.

## 5. Operatörler için öneriler

- Yalnızca HTTPS; `MUMWEB_ALLOWED_ORIGINS` ayarlı kalsın. İmajı sabitleyin: `.env` içinde `MUMWEB_VERSION=0.3.0` veya
  digest (`image: aoztrk82/mumweb@sha256:…`) ve bilerek güncelleyin (`docker compose pull`). `mumblevoip/mumble-server`
  ve `caddy` için de aynısı.
- Gateway Mumble'a kontrol etmediğiniz bir ağdan ulaşıyorsa Mumble sertifikasını sabitleyin:
  `MUMWEB_TLS_FINGERPRINT` (`mumprobe -addr host:64738 -exit` yazdırır). Compose ağında isteğe bağlıdır.
- Yönetim sayfasını `Caddyfile` içinde kendi adresinize kısıtlayın (`203.0.113.7`'yi değiştirin):

  ```caddy
  @blocked {
  	path /admin /admin/*
  	not remote_ip 203.0.113.7
  }
  respond @blocked 404
  ```

  Bu satırları site bloğunun içine, `reverse_proxy`'den önce koyun. Panel yalnızca o adresten kullanılabilir kalır.
  (Yerel Caddy'ye karşı denendi: `/` 200, başka adreslerden `/admin/` ve `/admin` 404 döndü.)
- Uzun, rastgele bir `MUMWEB_ADMIN_PASSWORD` ve farklı bir Mumble SuperUser şifresi kullanın.
- Sunucu dünyaya açık olmasın istiyorsanız Mumble'ın kendi sunucu şifresini veya kayıt kurallarını koruyun.
- Güncelleme politikası: her sürümden sonra `docker compose pull && docker compose up -d`; bakımcılar sürümden önce aşağıdaki
  kontrolleri yeniden çalıştırır ve önce HIGH/CRITICAL bulguları düzeltir.

## 6. Kontrolleri tekrarlama

```sh
cd gateway
go install golang.org/x/vuln/cmd/govulncheck@latest honnef.co/go/tools/cmd/staticcheck@latest github.com/securego/gosec/v2/cmd/gosec@latest
govulncheck ./... ; staticcheck ./... ; gosec -quiet -exclude-dir=proto ./...
go test ./... -count=1                                            # sınır, yönetim ve başlık testlerini içerir
go test ./internal/bridge -run '^$' -fuzz FuzzClientMessage -fuzztime 1m
cd ../web && npm audit && npm test                                 # XSS testlerini içerir
docker run --rm aquasec/trivy:latest image --severity HIGH,CRITICAL aoztrk82/mumweb:<sürüm>
```

Araçları projeyle aynı Go sürümüyle derleyin (`GOTOOLCHAIN=go1.27.2`), yoksa kodu okuyamazlar. Trivy'nin varsayılan veritabanı
aynası başarısız olursa `--db-repository ghcr.io/aquasecurity/trivy-db:2` ekleyin.

## 7. Henüz yapılmadı

- Frontend için Semgrep ve ESLint güvenlik kuralları; yukarıdaki taramaları çalıştıran bir CI işi.
- Distroless/scratch çalışma imajı (Alpine imajı yalnızca sağlık kontrolü için bir kabuk ve `wget` ekler; yerleşik bir
  `-healthcheck` bayrağı bunu kaldırır).
- Compose dosyalarında imaj digest sabitleme (kurulumlar bayatlamasın diye öneri olarak belgelendi).
- Caddy ve Mumble için salt okunur kök dosya sistemi (yazılabilir yollara ihtiyaçları var; açmadan önce denenmeli).
- Uzun fuzz koşuları; gerçek bir Mumble sunucusuna karşı protokol fuzzer'ı.
- Yük altında kapasite ve kötüye kullanım testi (TODO 5).
