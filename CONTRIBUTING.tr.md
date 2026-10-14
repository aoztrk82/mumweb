# Katkıda bulunma

[English](CONTRIBUTING.md) | **Türkçe**

mumweb'e katkıda bulunmayı düşündüğünüz için teşekkürler.

## Bu proje AI ile yazıldı, ve bu burada hoş karşılanıyor

mumweb'in gateway'i, web istemcisi ve dokümantasyonu, satır satır elle yazılmak yerine, proje sahibiyle birlikte
istek istek [Claude Code](https://claude.com/claude-code) tarafından yazıldı. [CLAUDE.md](CLAUDE.md) — Claude'u bu
depoda yönlendiren dosya — bu projenin sahip olduğu en yakın mimari belgesidir ve tam da bu yüzden güncel
tutuluyor.

AI destekli katkılar sadece hoş görülmüyor, açıkça isteniyor: Claude, Codex, Copilot veya başka bir asistanla
yazılmış bir pull request, elle yazılmış biriyle aynı ölçütle değerlendirilir — doğru, test edilmiş ve belgelenmiş
olup olmadığına göre, nasıl yazıldığına göre değil. Bir asistan kullandığınızı açıklamak zorunda değilsiniz, ama
isterseniz PR açıklamasında belirtebilirsiniz (örneğin hangi modeli kullandığınızı ve ondan ne istediğinizi);
her iki durumda da inceleme çıtası aynı. Kendi AI asistanınızı bu depoya getiriyorsanız önce `CLAUDE.md`'ye
yönlendirin — kurallar, dosya düzeni ve protokolle ilgili tuzaklar zaten orada, aksi halde asistanın bunları
yeniden keşfetmesi gerekir.

## Başlamadan önce

- **Hata ya da küçük düzeltme:** doğrudan bir pull request açın.
- **Yeni özellik veya davranış değişikliği:** önce bir issue açın, böylece siz (veya asistanınız) kodu yazmadan
  önce yaklaşım üzerinde anlaşabiliriz. [docs/TODO.md](docs/TODO.md)'ye bakın — zaten planlanmış veya değiştirilmesi
  planlanmayan bilinen bir sınır olarak işaretlenmiş olabilir.
- **Güvenlik sorunu:** herkese açık bir issue açmayın. [SECURITY.md](SECURITY.md)'yi izleyin (İngilizce; depo genel
  güvenlik politikası tek dilde tutulur).

## Değişiklik yaparken

1. Depoyu fork'layın ve değişiklik için bir dal (branch) oluşturun.
2. [CLAUDE.md](CLAUDE.md)'deki kurallara uyun: Go kodu `gofmt`'lanmış, Svelte 5 runes ile ve strict TypeScript,
   Conventional Commits (`feat:`, `fix(gateway):`, `docs:`, …), gerekçesiz yeni bağımlılık yok.
3. Değişikliği yerelde derleyip test edin — bkz. [docs/BUILDING.md](docs/BUILDING.md) ve
   [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md). PR açmadan önce en azından:
   - `gofmt -l gateway` boş dönmeli (Windows checkout'ları CRLF: `tr -d '\r' < f | gofmt -l`)
   - `go vet ./...` ve `make test` (Go + Vitest) geçmeli
   - `web/` içinde `npm run check` (TypeScript) geçmeli
4. **Dokümantasyonu aynı commit'te güncelleyin**, sonradan değil — CLAUDE.md'nin "Keep the documentation up to
   date" bölümündeki tablo, hangi değişiklik için hangi dosyanın güncellenmesi gerektiğini tam olarak söylüyor
   (yeni bir ortam değişkeni veya özellik için README tabloları, doğrulanan veya keşfedilen her şey için
   `docs/PROGRESS.md`, maddeleri işaretlemek veya eklemek için `docs/TODO.md`, vb.). Dokümantasyon değişiklikleri
   **her iki dilde** gerekir: önce İngilizce, sonra `docs/tr/` içindeki Türkçe karşılığı (kök dosyalar için
   `README.tr.md` / `CONTRIBUTING.tr.md`). Bir şeyi düzgün çevirmiyorsanız, sessizce eski bırakmak yerine bunu
   Türkçe dosyada belirtin — Türkçe okuyabilen bir asistan genelde bu çeviri adımını sizin için yapabilir.
5. Değişikliğiniz bir arayüz metnine dokunuyorsa, `web/src/i18n/` içindeki **yedi** sözlüğün hepsine ekleyin
   (`tr.ts` şekli tanımlar; bir dilde anahtar eksikse `npm run check` başarısız olur). Yedi dilin hepsini
   bilmeniz gerekmiyor — bilmediğiniz bir dile AI asistanıyla çeviri yaptırmak sorun değil, ama tonu ve
   SuperUser/ACL/Opus gibi özel adların mevcut sözlüklerle uyumlu şekilde Latin alfabesinde kalmasını bir konuşucuya
   (varsa) veya asistana kontrol ettirin.
6. Pull request'i `main`'e karşı açın. Ne değiştiğini ve nedenini anlatın; varsa çözdüğü issue'ya bağlantı verin.

## İncelemede nelere bakılır

- Doğruluk ve test kapsamı, ilgiliyse `gateway/internal/bridge` içindeki Go entegrasyon testleri dahil (çalışan bir
  Mumble sunucusu gerektirir, bkz. [docs/BUILDING.md](docs/BUILDING.md)).
- Güvenlik gerilemesi yok: gateway yalnızca tek yapılandırılmış Mumble adresine bağlanır, Origin/oturum/hız
  sınırları bozulmadan kalır, hassas hiçbir şey (şifreler, sertifika anahtarları, sohbet içeriği) loglanmaz. Tam
  tehdit modeli için [docs/SECURITY.md](docs/SECURITY.md)'ye bakın.
- Dokümantasyonun, yukarıda açıklandığı gibi her iki dilde güncel tutulması.
- Kapsam: dağınık bir değişiklik yerine odaklı bir değişiklik. Bir PR, kaynaklandığı issue'nun ötesine büyüdüyse
  bölünmesi sorun değil.

## Davranış kuralları

Saygılı olun ve iyi niyeti varsayın — AI destekli olduğunu anlayabildiğiniz katkılar için de, çünkü bu kod tabanının
büyük kısmı da öyle yazıldı. Bir yaklaşıma dair anlaşmazlıklar issue veya PR başlığında, teknik gerekçelerle
tartışılsın.
