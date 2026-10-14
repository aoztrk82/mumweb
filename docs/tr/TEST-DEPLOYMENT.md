# Test dağıtımı (1-2 gün)

[English](../TEST-DEPLOYMENT.md) | **Türkçe**

Gerçek production kurulumundan önce stres ve güvenlik testleri için açılan kısa ömürlü, herkese açık bir örnek.
Production yığınını daha gevşek limitlerle çalıştırır (`deploy/test/`). Bu sayfa, sunucudaki bir Claude Code oturumunun (veya
bir kişinin) başka hiçbir bağlam olmadan kurulumu yapabilmesi için yazıldı.

## Ne kuruyorsunuz

mumweb = Mumble sunucusu + Go gateway + Caddy (otomatik HTTPS); hepsi Docker Compose içinde:

```
Tarayıcı ⇄ HTTPS (Caddy :443) ⇄ gateway (yalnızca iç ağ) ⇄ Mumble sunucusu (:64738)
```

İmajlar Docker Hub'dan gelir (`aoztrk82/mumweb`, `mumblevoip/mumble-server`, `caddy`); sunucuda derleme gerekmez. Test sitesi
1-2 gün açık kalacak, sonra gerçek production yapılandırmasıyla değiştirilecek.

## Production'dan farkları

| Ayar | Production dosyası (`deploy/docker-compose.yml`) | Test dosyası (`deploy/test/compose.yaml`) |
|---|---|---|
| Web oturumu (`MUMWEB_MAX_SESSIONS`) | 50 | 200 |
| IP başına oturum (`MUMWEB_MAX_SESSIONS_PER_IP`) | 10 (gateway varsayılanı) | 100 |
| IP başına dakikada yeni bağlantı (`MUMWEB_CONN_RATE`) | 10 (gateway varsayılanı) | 600 |
| Mumble kullanıcı sınırı | 50 | 200 |
| Gateway log seviyesi | info | debug |
| Gateway bellek / CPU / işlem | 256 MiB / 1 / 200 | 512 MiB / 2 / 400 |
| Geri kalan her şey (salt okunur gateway, çıkış yok, capability'ler kapalı, HSTS, CSP) | aynı | aynı |

IP başına gevşek sayılar, testlerin tek makineden gelmesi için var. Production'da **bırakmayın**.

## Ön koşullar (eksik olanı sahibinden isteyin)

1. Docker Engine ve Compose eklentisi (`docker compose version`) ile `git` kurulu bir Linux sunucu.
2. Test sitesi için, A/AAAA kaydı sunucunun genel IP'sine yönelen bir alan adı.
3. Sunucu ve bulut güvenlik duvarında açık portlar: **80/tcp, 443/tcp, 443/udp** (HTTPS), **64738/tcp ve 64738/udp**
   (masaüstü Mumble istemcileri; yalnızca web testleri için şart değil).
4. `.env` değerleri (aşağıda). Sahibinden isteyin; şifre uydurmayın ve commit etmeyin.

## Adımlar

```sh
git clone https://github.com/aoztrk82/mumweb.git
cd mumweb/deploy/test
cp .env.example .env        # düzenleyin: alan adı ve şifreler (chmod 600 .env)
docker compose pull
docker compose up -d
docker compose ps           # üç servis çalışıyor
curl -s https://ALAN-ADINIZ/healthz     # ok
curl -s https://ALAN-ADINIZ/config      # {"accessPassword":true,"admin":true,"reservedSlots":true}
```

`.env` değerleri: `MUMWEB_DOMAIN`, `MUMBLE_SUPERUSER_PASSWORD`, `MUMWEB_ACCESS_PASSWORD`, `MUMWEB_ADMIN_PASSWORD`, isteğe bağlı
`MUMWEB_RESERVED_SLOTS` + `MUMWEB_RESERVED_PASSWORD`. Dosya git tarafından yok sayılır. Caddy TLS sertifikasını ilk
başlangıçta alır; 80 ve 443 portlarına erişilebilmeli ve DNS sunucuya yönelmiş olmalı (`docker compose logs caddy` ile izleyin).

Ayrılmış slotlar `0.3.1` veya daha yeni imaj ister; `latest` eski bir sürümdeyse değişkenler yok sayılır ve `/config`
`"reservedSlots":false` gösterir. Böyle olursa sahibine söyleyin.

## Kurulumdan sonra kontroller

1. `https://ALAN-ADINIZ/` adresini açın, erişim şifresini ve bir takma ad girin: Mumble kanal ağacına girmelisiniz.
2. `https://ALAN-ADINIZ/admin/` yönetim şifresini ve SuperUser şifresini ister; sekmeler yüklenir.
3. Yanıt başlıklarında `Content-Security-Policy` ve `Strict-Transport-Security` var:
   `curl -sI https://ALAN-ADINIZ/ | grep -i -E 'content-security|strict-transport'`.
4. Gateway internete çıkamaz: `docker compose exec mumweb wget -T 4 -O- http://1.1.1.1` başarısız olur;
   `docker compose exec mumweb touch /x` başarısız olur (salt okunur).

## İşletme

| İş | Komut (`deploy/test` içinde) |
|---|---|
| Loglar (gateway) | `docker compose logs -f --tail=200 mumweb` |
| Loglar (hepsi) | `docker compose logs --since 1h > /tmp/mumweb-logs.txt` |
| Kaynak kullanımı | `docker stats --no-stream` |
| En yeni imaja güncelleme | `docker compose pull && docker compose up -d` |
| Sürüm sabitleme | `.env` içine `MUMWEB_VERSION=x.y.z`, sonra `docker compose up -d` |
| Durdur, veriyi koru | `docker compose down` |
| Mumble verisi ve sertifikalar dahil her şeyi sil | `docker compose down -v` |

Gateway kullanıcı adlarını, IP'leri ve olayları loglar; şifre, sertifika anahtarı veya sohbet metnini asla. Sorun bildirirken
ilgili log satırlarını, `docker compose ps` çıktısını ve `docker stats` değerlerini gönderin; `.env` yapıştırmayın.

## Bunu işleten kişi/oturum için kurallar

- Bir şeyi "düzeltmek" için sertleştirmeyi (`read_only`, `cap_drop`, `backend` iç ağı) değiştirmeyin; bildirin. Test edilenlerin
  bir parçası.
- Gateway portunu (8080) veya Mumble'ın Ice portunu yayınlamayın; yalnızca Caddy (80/443) ve Mumble (64738) dışarıya açık.
- Stres ve güvenlik testlerini sahibinin diğer oturumu bu adrese karşı çalıştırır. Loglarda tek IP'den çok bağlantı, yanlış
  şifre denemeleri ve kilitlenmeler görmeyi bekleyin; bu test, saldırı değil. Bunun dışındaki her olağandışı durumu bildirin.
- Sunucudaki depoda dosya düzenlemeyin; kodu sahibi başka yerde değiştirir, siz `git pull` + `docker compose pull` yaparsınız.

## Gerçek production'a geçiş

Testlerden sonra sahibi nihai limitleri belirler. Sonra `deploy/docker-compose.yml`'e (README hızlı başlangıçtaki production
varsayılanları), yeni bir `deploy/.env`'e (bkz. `deploy/.env.example`), sabitlenmiş imaj sürümlerine ve istenirse `/admin`'i tek
adrese kısıtlayan Caddy kuralına geçilir ([SECURITY.md](SECURITY.md) bölüm 5). Aynı portlarda production yığınını başlatmadan
önce `docker compose -f deploy/test/compose.yaml down` çalıştırın (test verisini silmek için `-v` ekleyin).
