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

### 3. Web yönetim paneli (mümkünse)
- [ ] Kapsam kararı. Mumble protokolüyle yapılabilenler: ACL ve gruplar, yasak listesi (görüntüle/ekle/kaldır),
      kayıtlı kullanıcılar (UserList: listele/yeniden adlandır/sil), kanal yönetimi (var), kullanıcı yönetimi (var).
      Sunucu ayarları (karşılama metni, bant genişliği, kullanıcı sınırı) protokolde yok; Ice/gRPC veya
      config + yeniden başlatma gerekir.
- [ ] Kimlik doğrulama: SuperUser veya admin grubundaki kayıtlı kullanıcı; production'da ayrı bir yönetici şifresi ve
      yalnızca HTTPS.
- [ ] Ayrı bir sayfa (`/admin`), ana arayüzden bağımsız; tüm işlemler loglanır.
- [ ] Masaüstü istemcide de olmayan ACL şifre alanı ve kanal şifresi zaten var; panelde de gösterilmeli.

### 4. Ayrıntılı güvenlik incelemesi (production'dan ÖNCE, zorunlu)
Hedef: hiçbir girdi yoluyla container dışına veya sunucu sistemine erişim sağlanamamalı.
- [ ] Tehdit modeli: internetten anonim kullanıcı, kötü niyetli web kullanıcısı, kötü niyetli Mumble kullanıcısı
      (gateway'e zararlı Mumble mesajı), çalınmış tarayıcı verisi.
- [ ] Container sertleştirme: `read_only: true`, `cap_drop: [ALL]`, `security_opt: no-new-privileges`, tmpfs,
      kaynak sınırları (`mem_limit`, `pids_limit`, `cpus`), seccomp varsayılan profili, root olmayan kullanıcı (var),
      imaj sürümlerinin sabitlenmesi (`latest` yerine sürüm + digest), mümkünse distroless/scratch çalışma imajı.
- [ ] Ağ: gateway yalnızca Mumble'a çıkabilsin (ayrı iç ağ, dışarı çıkış yok); Mumble yönetim (Ice) portu kapalı.
- [ ] Bağımlılık taraması: `govulncheck`, `npm audit`, imaj taraması (Trivy/Grype); güncelleme politikası.
- [ ] Statik analiz: `gosec`, `staticcheck`, Semgrep; frontend için ESLint güvenlik kuralları.
- [ ] Gateway protokol fuzzing: WS JSON ve ikili çerçeveler, Mumble'dan gelen protobuf mesajları (go-fuzz / `testing.F`).
- [ ] DoS: mesaj boyutu/oranı sınırları (var) gözden geçirilecek, yavaş istemci (slowloris) ve bellek tüketimi,
      oturum başına kuyruk sınırları, ses paketi flood'u.
- [ ] Web: Content-Security-Policy, HSTS, `X-Frame-Options`/`frame-ancestors`, Referrer-Policy, Permissions-Policy
      (Caddy veya gateway); sohbet/açıklama HTML'i (DOMPurify) için XSS testleri.
- [ ] Tarayıcıda saklanan özel anahtar (IndexedDB) ve kayıtlı kanal şifreleri (localStorage): XSS olursa çalınabilir;
      non-extractable WebCrypto anahtarı veya şifreleme değerlendirmesi.
- [ ] Loglarda gizli bilgi olmadığının denetimi (şifre, anahtar, token, sohbet).
- [ ] TLS: Caddy ayarları, gateway ↔ Mumble için parmak izi sabitleme önerisi.
- [ ] Bilinen protokol riskleri: masaüstünden yapılan IP ban (bkz. C), SuperUser'ın web'den kullanımı (kapalı),
      Mumble mesaj hız sınırı.
- [ ] Bulgular `docs/SECURITY.md`'ye; güvenlik açığı bildirimi yolu (SECURITY policy).

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
- [ ] [TESTING.md](TESTING.md)'deki 12 maddelik manuel kabul listesi.

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
