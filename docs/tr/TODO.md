# Yapılacaklar (TODO)

[English](../TODO.md) | **Türkçe**

Son güncelleme: 2026-10-14. Bu sadece özellik yol haritası. Yapılan işler ve doğrulama geçmişi:
[PROGRESS.md](PROGRESS.md). Güvenlik incelemesi ve sıkılaştırma: [SECURITY.md](SECURITY.md). Manuel kabul listesi:
[TESTING.md](TESTING.md).

## Planlanan özellikler

- Diğer tarayıcı dilleri için varsayılanın Türkçe yerine İngilizce olup olmayacağına karar ver
  (`web/src/i18n/index.svelte.ts` içinde `FALLBACK`); başka diller eklenecek mi?
- Reddit duyurusu için README cilası: web arayüzü ekran görüntüleri / kısa demo GIF'i, sürüm ve CI rozetleri, diğer
  Mumble web istemcileriyle "Neden mumweb?" karşılaştırması.
- Davet bağlantısı (imzalı, süreli token), erişim şifresine alternatif veya ek olarak.
- Kendi açıklamasını düzenleme; avatar gösterme/yükleme.
- Kanal açıklamasını görüntüleme (düzenleme var).
- Fısıltı/bağırma (whisper/shout, VoiceTarget).
- Masaüstünden `.p12` sertifika içe aktarma (şu an yalnızca PEM).
- Tarayıcı bildirimleri (sekme arka plandayken mesaj/katılım), sekme başlığında okunmamış sayısı.
- Mobilde Screen Wake Lock, ekran kapanınca ses kesilmesini azaltmak için.
- Android'de arka planda düşme sorununu çöz: başka uygulamaya geçince kullanıcı 15–20 sn sonra odadan düşüyor (ekran
  kilidi sorun değil). Sekmenin dondurulup dondurulmadığı mı yoksa soketin mi öldürüldüğü araştırılacak; arka
  plandayken daha kısa ping ve/veya `visibilitychange` ile hızlı yeniden bağlanma denenebilir.
- Firefox Android girişte "codec desteklenmiyor" diyor; `AudioEncoder`/`AudioDecoder` ya da Opus yapılandırmasından
  hangisinin eksik olduğunu bulup kesin bir mesaj göster.
- Sohbete resim yapıştırma (Mumble `data:` URI resimleri, `ImageMessageLength` sınırı).
- Gelişmiş ses ayarları: bitrate, jitter buffer, gürültü bastırma seçenekleri; kanal dinleyicilerine yerel ses ayarı
  (`listening_volume_adjustment`).
- Yönetim paneli: kullanıcıları panelden kanallar arasında taşıma, kalıcı bir denetim log dosyası, tek ortak
  yönetici şifresi yerine bir admin grubunu panel erişimine bağlama.
- Yönetim paneli: yasak listesi yazımını diff tabanlı yap (taze çekilmiş listeye karşı anahtarla ekle/sil), tüm
  listeyi değiştirmek yerine — böylece bayat bir panel sekmesi başka yerden eklenen bir yasağı geri alamaz.
- Kapasite: repoda bir yük testi aracı, VPS boyutuna göre önerilen `MUMWEB_MAX_SESSIONS` tablosu, ve yeniden bağlanma
  döngüsü yerine arayüzde anlaşılır bir "sunucu dolu" mesajı (kapasiteyi `/config` / `/healthz` ile bildir).

## Bilinen sınırlar (değiştirilmesi planlanmıyor)

- Masaüstü istemciden yasaklanan web kullanıcısı gateway IP'sini de yasaklatır, yani tüm web kullanıcılarını. Web
  arayüzünün kendi "Yasakla…" işlemi sadece sertifikayı yasaklayarak bunu önler. Genel bir çözüm planlanmıyor (küçük
  gruplar); gerçek bir sorun olursa "bekçi hesabı" gateway IP'sinin yasağını otomatik kaldırabilir.
- Tarayıcı sekmesi odakta değilken global bas-konuş tuşu mümkün değil (tarayıcı sınırı); oyun sırasında ses algılama
  veya masaüstü istemci kullanılmalı.
- Mumble sunucusu kullanıcı başına mesaj hızını sınırlar (1/sn, 5 burst) ve fazlasını sessizce düşürür — masaüstünde
  de aynı.
- SuperUser Mumble'da konuşamaz, tasarım gereği.
- Eski Safari/iOS için WASM Opus yedeği yapılmayacak.
