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

Yönetim paneli (`MUMWEB_ADMIN_PASSWORD` ayarlı; yerel ortamda `admin`):

13. Değişken ayarlı değilken `/admin` girişten sonra "kapalı" der. Ayarlıyken: yanlış yönetici şifresi reddedilir, 5
    yanlış denemeden sonra IP bir süre kilitlenir; ana giriş yine çalışır.
14. Ana istemci başka bir sekmede normal bir takma adla bağlıyken, aynı tarayıcıda `/admin/`'e `SuperUser` ile girin: ikisi
    de bağlı kalır (kimse atılmaz) ve panel **Çevrimiçi** altında sesli kullanıcıyı listeler. Ses sekmesinde konuşun; ses
    eskisi gibi çalışır.
15. **Kanallar:** kanal oluşturun, yeniden adlandırın ve silin; kanal şifresi koyun ve normal kullanıcıya şifre
    sorulduğunu görün; **ACL ve gruplar**'ı açın, bir izni işaretleyin, bir grup üyesi ekleyin, kaydedin, yeniden açın ve
    değişikliği görün. Masaüstü istemcinin ACL düzenleyicisinde aynı ACL görünür.
16. **Yasaklar:** **Çevrimiçi**'den bir kullanıcıyı yasaklayın (yalnızca sertifika), kaydı görün, silip kaydedin;
    kullanıcı yeniden girebilir. Masaüstünden yapılmış bir yasak (IP + özet) burada silinebilir.
17. **Kayıtlı kullanıcılar:** **Çevrimiçi**'den bir kullanıcıyı kaydedin, panelde yeniden adlandırın ve silin. Art arda
    on hızlı işlem yapın ve hiçbirinin sessizce kaybolmadığını kontrol edin (yerine "çok fazla işlem" mesajı çıkabilir).
