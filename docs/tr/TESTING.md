# Manuel kabul testleri

[English](../TESTING.md) | **Türkçe**

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
