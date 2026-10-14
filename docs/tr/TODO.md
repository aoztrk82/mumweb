# Yapılacaklar (TODO)

[English](../TODO.md) | **Türkçe**

Son güncelleme: 2026-10-13. Ayrıntılı geçmiş ve doğrulama notları: [PROGRESS.md](PROGRESS.md).

## A. Öncelikli işler (kullanıcının belirlediği sırayla)

### 1. Projenin ana dili İngilizce olacak
- [x] `README.md`, `docs/DEVELOPMENT.md`, `docs/TESTING.md`, `docs/PROGRESS.md`, `docs/TODO.md` İngilizce ana sürüm;
      Türkçe sürümler `README.tr.md` ve `docs/tr/*`, her sayfa karşılığına bağlantı veriyor.
- [x] Deploy dosyalarındaki Türkçe değerler (compose karşılama metni, `.env.example`, `certs` hata mesajı).
- [x] Commit mesajları ve kod yorumları İngilizce (kontrol edildi; kodda kalan Türkçe yalnızca arayüz metinleri).
- [x] **Arayüz dili.** `tr.ts` yanında `web/src/i18n/en.ts`; dil tarayıcıya göre seçilir (İngilizce de Türkçe de
      değilse Türkçe), giriş ekranında ve durum çubuğunda EN / TR düğmesi var. Statik `index.html` /
      `manifest.webmanifest` metinleri İngilizce.
- [ ] Diğer tarayıcı dilleri için varsayılanın Türkçe yerine İngilizce olup olmayacağına karar ver
      (`web/src/i18n/index.svelte.ts` içinde `FALLBACK`); başka diller eklenecek mi?
- [ ] `MUMWEB_PLAN.md` repoya eklenecekse İngilizceye çevrilsin ve `frame_number` ifadesi "+2 / 20 ms" olarak düzeltilsin.
- [ ] Reddit duyurusu öncesi README cilası: web arayüzü ekran görüntüleri / kısa demo GIF'i, sürüm ve CI rozetleri,
      diğer Mumble web istemcileriyle "Neden mumweb?" karşılaştırması.

### 2. Web erişim şifresi (sunucu şifresinden ayrı)
- [x] `MUMWEB_ACCESS_PASSWORD` (boşsa kapalı): web arayüzü için gereken, yalnızca gateway'in bildiği şifre; Mumble
      sunucu şifresinden bağımsız.
- [x] `hello`'da Mumble'a bağlanmadan önce, sabit zamanlı karşılaştırmayla ve ayrı `reject` nedenleriyle kontrol edilir.
- [x] Kaba kuvvet koruması: yanlış denemede 1 sn gecikme, 5 yanlış denemede IP 10 dakika engellenir.
- [x] Giriş ekranı alanı yalnızca `/config` isterse gösterir; "bu tarayıcıda hatırla" var.
- [ ] Tarayıcıda elle denenmedi (yalnızca otomatik gateway testleri ve tip kontrolü yapıldı).
- [ ] Değerlendirme: şifre yerine/yanında davet bağlantısı (imzalı, süreli token) daha kullanışlı olabilir.

### 3. Web yönetim paneli (2026-10-14'te yapıldı)
- [x] Kapsam: çevrimiçi kullanıcılar (at, sertifika yasağı, sustur/sağır et, kaydet), kanal ağacı (oluştur, yeniden
      adlandır, sil, kanal şifresi), tam ACL ve grup düzenleyicisi, yasak listesi (her kaydı silme, sertifika yasağı
      ekleme), kayıtlı kullanıcılar (listele, yeniden adlandır, sil). Sunucu ayarları (karşılama metni, bant genişliği,
      kullanıcı sınırı) protokolde yok; Ice/gRPC veya config + yeniden başlatma gerekir, kapsam dışı kaldı.
- [x] Kimlik doğrulama: `MUMWEB_ADMIN_PASSWORD` (boşsa panel kapalı) ve giriş formunda bir Mumble hesabı
      (`MUMWEB_ALLOW_SUPERUSER` ana arayüzde kapalı olsa da SuperUser burada çalışır); kendi kilit sayacı.
- [x] Ayrı sayfa (`/admin/`), kendi kimliği (sertifika anahtarı `cert-admin`), ses ve mikrofon yok; bu sayede sesli
      istemci ve panel aynı tarayıcıda yan yana çalışır; durumu değiştiren her işlem loglanır.
- [x] Panelde kanal şifresi ve ACL düzenleme.
- [x] Gerçek bir Mumble 1.5.915 sunucusuna karşı (Go entegrasyon testi) ve yerel kurulumda başsız Chrome ile
      doğrulandı (bkz. PROGRESS).
- [ ] Sahibi tarafından elle henüz denenmedi; telefonda yerleşimi kontrol edin.
- [ ] Yasak listesi yazımı tüm listeyi değiştirir; bu yüzden Yasaklar sekmesi açıkken başka yerden eklenen bir yasağı
      bayat bir panel geri alabilir (sekme her açılışta ve her kayıttan sonra yeniden okur). Sağlam çözüm: ekleme/silmeyi
      anahtarla gönderip gateway'in taze çekilmiş listeye uygulaması (`completeBans` gibi).
- [ ] Fikirler: panelden kullanıcıları kanallar arasında taşıma, kalıcı bir denetim log dosyası, tek ortak yönetici
      şifresi yerine bir admin grubunu panel erişimine bağlama, masaüstü IP yasakları için "bekçi hesabı" (bkz. C).
- [x] `0.2.0` olarak yayınlandı; Docker Hub sayfası yenilendi.

### 4. Ayrıntılı güvenlik incelemesi (ilk tur 2026-10-14'te yapıldı; sonuçlar [SECURITY.md](SECURITY.md)'de)
Hedef: hiçbir girdi yoluyla container dışına veya sunucu sistemine erişim sağlanamamalı.
Yapılanlar (ayrıntı ve gerekçe SECURITY.md'de): tehdit modeli; gateway `read_only` + `cap_drop: ALL` + `no-new-privileges` +
bellek/işlem/CPU sınırları, Caddy ve Mumble için `no-new-privileges` ve sınırlar (Mumble'ın giriş betiği `cap_drop: ALL` ile
bozuluyor); gateway `internal` ağda, internete yolu yok (doğrulandı); bir kez `govulncheck`, `npm audit` ve yayınlanan imajda
Trivy: temiz; `gosec` (3 kabul edilen bulgu) ve `staticcheck`; `testing.F` ile 5 fuzz hedefi (25'er sn, çökme yok); IP başına
eşzamanlı oturum sınırı, kontrol mesajı hız sınırı, HTTP boşta/başlık sınırları; CSP, `X-Frame-Options`, Referrer-Policy,
Permissions-Policy, COOP (gateway) ve HSTS (Caddy); 21 XSS yük testi; tarayıcıdaki özel anahtar değerlendirildi (gateway PEM'e
ihtiyaç duyduğu için çıkarılamaz anahtar mümkün değil, kabul edilen risk); log denetimi (sır yok); `docs/SECURITY.md` ve kök
`SECURITY.md` politikası.
Kalanlar:
- [ ] Compose dosyalarında imaj digest sabitleme (şimdilik operatör önerisi), distroless/scratch gateway imajı (yerleşik sağlık
      kontrolü gerekir), Caddy ve Mumble için salt okunur kök.
- [ ] Taramaları tekrarlayan CI işi; frontend için Semgrep ve ESLint güvenlik kuralları.
- [ ] Daha uzun fuzz koşuları ve gerçek sunucuya karşı protokol fuzzer'ı.
- [x] Sahibi GitHub depo ayarlarında **Private vulnerability reporting**'i açtı (2026-10-14); politika oraya yönlendiriyor.
- [x] Çalışan bir test dağıtımına canlı kara kutu testi (2026-10-14, `mumweb.aozturk.site`, 0.3.1 sürümü): başlıklar/TLS,
      açık portlar, Origin kontrolü, kilitlemeler, IP başına sınır, aşırı büyük/bozuk çerçeveler, kontrol ve ses selleri,
      boşta zaman aşımları, sohbet seli, kullanıcı adı alanında XSS, eşzamanlı giriş yükü. Her şey tuttu; bkz. SECURITY.md.

### 5. Kapasite testi ve kullanıcı sınırı
- [ ] Yük testi aracı (repoda): N tarayıcı-benzeri oturum, değişen konuşan sayısı (1, 2, 5, hepsi), süre; gateway ve
      Mumble CPU/RAM/ağ ölçümü. İlk ölçüm (10 oturum, hepsi konuşuyor): gateway ~%45 tek çekirdek, ~15 MiB.
- [ ] Hedef VPS boyutları için tablo (ör. 1 vCPU/1 GB, 2 vCPU/2 GB) ve önerilen `MUMWEB_MAX_SESSIONS`.
- [ ] Gerekirse optimizasyon: ses çerçevesi başına `context.WithTimeout` + WS yazımı, JSON yerine ikili kontrol mesajları,
      tampon havuzu.
- [ ] Arayüz: sunucu dolu olduğunda (gateway WS'yi 503 ile reddediyor) şu an yeniden bağlanma döngüsüne giriyor;
      "Web istemcisi dolu (N/N)" gibi anlaşılır mesaj ve döngüyü durdurma. Gateway kapasiteyi `/config`/`/healthz`
      ile bildirebilir.
- [ ] Mumble sunucusunun kendi kullanıcı sınırıyla (`MUMBLE_CONFIG_USERS`) uyum.

## B. Bekleyen manuel testler (kullanıcı)
- [x] Masaüstü Mumble ↔ web arasında **gerçek mikrofonla** konuşma iki yönde (sahibi, 2026-10-13: PC ↔ Android telefon,
      ses ve mesaj iki yönde).
- [ ] Bas-konuş bırakılınca masaüstündeki konuşma göstergesinin hemen sönmesi.
- [ ] Android, kalan kontroller (D'deki 2026-10-13 bulgularına bakın): **güvenilir** bir sertifikayla "Ana ekrana ekle" ve
      tam ekran açılış (telefon yerel mkcert CA'ya güvenmediği için ekleme seçeneği hiç çıkmadı), telefonda bas-konuş,
      Android Chrome (test Samsung Internet ile yapıldı).
- [ ] Kulaklıksız kullanımda yankı (tarayıcı AEC'si).
- [ ] Firefox (Windows) gerçek mikrofon; Safari 26+ (iOS/macOS) — WebKit kaynağına göre WebCodecs Opus var, cihazda denenmedi.
- [ ] [TESTING.md](TESTING.md)'deki 20 maddelik manuel kabul listesi (13–17. maddeler: yönetim paneli, 18–20.: güvenlik kontrolleri).

## C. Bilinen sınırlar ve ertelenen kararlar
- Sahibinin Android'de bulduğu (2026-10-13, Samsung Internet): ses ve mesaj çalışıyor, mobil görünüm iyi, ekran
  kilitlenince oturum sürüyor. İki sorun kaldı, ikisi de D'de: Firefox Android 157.0.1 codec desteklenmiyor diyor ve
  tarayıcı arka plana alınınca 15–20 sn sonra oturum odadan düşüyor.
- Masaüstünden yasaklanan web kullanıcısı gateway IP'sini de yasaklatır (yeni web girişleri engellenir). Karar:
  şimdilik önlem yok (küçük gruplar). Gerekirse "bekçi hesabı" ile otomatik düzeltme.
- Tarayıcı odakta değilken global bas-konuş tuşu mümkün değil (tarayıcı sınırı); oyun sırasında ses algılama veya
  masaüstü istemci önerilir.
- Mumble sunucusu kullanıcı başına mesaj hızını sınırlar (1/sn, 5 burst) ve fazlasını sessizce düşürür.
- SuperUser Mumble'da konuşamaz (tasarım gereği).
- WASM Opus yedeği (eski Safari/iOS) yapılmayacak.
- Production kurulumu (kullanıcının sunucusundaki mevcut Caddy + mumble-server) uygulama tamamlanınca; o sunucuda
  `autobanSuccessfulConnections=false` ayarı gerekecek.

## D. Eksik özellikler (masaüstü istemciye göre)
- [ ] Kendi açıklamasını düzenleme; avatar gösterme/yükleme.
- [ ] Kanal açıklamasını görüntüleme (düzenleme var).
- [ ] Fısıltı/bağırma (whisper/shout, VoiceTarget).
- [ ] Kayıtlı kullanıcılar listesi, yasak listesi yönetimi, ACL/grup düzenleyici (bkz. A3).
- [ ] Masaüstünden `.p12` sertifika içe aktarma (şu an yalnızca PEM).
- [ ] Tarayıcı bildirimleri (sekme arka plandayken mesaj/katılım), sekme başlığında okunmamış sayısı.
- [ ] Mobilde Screen Wake Lock (ekran kapanınca ses kesilmesini azaltmak için).
- [ ] **Android'de arka planda düşme:** başka uygulamaya geçince 15–20 sn sonra kullanıcı odadan düşüyor (ekran kilidi
      sorun değil). Sekmenin dondurulup dondurulmadığı (20 sn'lik ping durur, gateway 60 sn mesaj gelmeyince oturumu
      kapatır) mı yoksa soketin mi öldürüldüğü araştırılacak; yüklü PWA, Android Chrome ve ön plan/medya oturumuyla
      denenecek; arka plandayken daha kısa ping, `visibilitychange` ile hızlı yeniden bağlanma ve kullanıcıyı
      bilgilendirme düşünülebilir. Yalnızca varsayım, henüz ölçüm yok.
- [ ] **Firefox Android 157.0.1:** girişte "codec desteklenmiyor". `AudioEncoder` / `AudioDecoder` ya da Opus
      yapılandırmasından hangisi eksik (kontrol `encoderSupported()` ve alıcı başlatma) bakılacak, kesin bir mesaj
      gösterilecek, sonuç tarayıcı tablosuna yazılacak. Firefox masaüstü destekleniyor.
- [ ] "Ana ekrana ekle" denemesi gerçek bir sertifikayla (ör. Let's Encrypt alan adı veya tünel); mkcert kök sertifikası
      telefona kurulmadıkça güvenilmez (docs/tr/DEVELOPMENT.md bölüm 10).
- [ ] Sohbete resim yapıştırma (Mumble data: URI resimleri; `ImageMessageLength` sınırı).
- [ ] Davet bağlantısı: kanal + önerilen takma ad (ve A2 erişim şifresi açıksa token).
- [ ] Gelişmiş ses ayarları: bitrate, jitter buffer, gürültü bastırma seçenekleri; kanal dinleyici ses ayarı.
- [ ] Kanal dinleyicilerine yerel ses ayarı (`listening_volume_adjustment`).

## E. Teknik borç
- [ ] Tarayıcı uçtan uca test betikleri (şu an geliştiricinin geçici klasöründe, puppeteer-core + browserless):
      repoya alınması (Playwright vb.) ve CI'da çalıştırılması.
- [ ] CI: `go vet`/`go test`, `npm run check`/`npm test`, Docker build; dev Mumble ile entegrasyon testleri.
- [x] Docker Hub imajı `aoztrk82/mumweb` (amd64 + arm64), `make publish`, compose onu çeker.
- [ ] Sürümleme: gateway `release` sabiti (`mumweb 0.1`), git etiketleri ve CHANGELOG; imaj yayınını CI'da otomatikleştir
      (etiket → derle → gönder).
- [x] Docker Hub sayfası: açıklama ve README (`docs/DOCKERHUB.md`, `make hub-readme`).
- [ ] arm64 imajını gerçek donanımda dene; yeni README kurulum adımlarını temiz bir sunucuda baştan sona çalıştır.
