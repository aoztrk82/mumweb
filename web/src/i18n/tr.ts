// Turkish UI strings. All user-visible text lives here.
export const tr = {
  appName: 'mumweb',

  login: {
    title: 'Sesli sohbete katıl',
    nickname: 'Takma ad',
    password: 'Sunucu şifresi',
    passwordHint: 'İsteğe bağlı',
    connect: 'Bağlan',
    connecting: 'Bağlanıyor…',
  },

  status: {
    idle: 'Bağlı değil',
    connecting: 'Bağlanıyor…',
    connected: 'Bağlı',
    reconnecting: (attempt: number, seconds: number) =>
      `Bağlantı koptu. Yeniden bağlanılıyor (${attempt}. deneme, ${seconds} sn)…`,
  },

  controls: {
    mute: 'Mikrofonu kapat',
    unmute: 'Mikrofonu aç',
    deafen: 'Sesi kapat',
    undeafen: 'Sesi aç',
    disconnect: 'Çıkış',
  },

  channels: {
    title: 'Kanallar',
    joinHint: 'Kanala geçmek için çift tıklayın',
    you: '(sen)',
    registered: 'Kayıtlı kullanıcı',
    muted: 'Susturulmuş',
    deafened: 'Sağırlaştırılmış',
    serverMuted: 'Sunucu tarafından susturulmuş',
    serverDeafened: 'Sunucu tarafından sağırlaştırılmış',
    speaking: 'Konuşuyor',
    volume: 'Ses seviyesi',
    localMute: 'Benim için sustur',
    resetVolume: 'Sıfırla',
  },

  audio: {
    unsupported: 'Tarayıcınız Opus ses kodlamayı desteklemiyor; ses çalışmayacak. Güncel Chrome, Edge veya Firefox kullanın.',
  },

  chat: {
    title: 'Sohbet',
    placeholder: (channel: string) => `#${channel} kanalına mesaj yaz`,
    send: 'Gönder',
    private: 'özel',
    joined: (name: string) => `${name} kanala katıldı`,
    left: (name: string) => `${name} kanaldan ayrıldı`,
    you: 'Sen',
    unknownUser: 'Bilinmeyen',
    server: 'Sunucu',
  },

  errors: {
    generic: 'Bir hata oluştu.',
    mumbleUnavailable: 'Mumble sunucusuna ulaşılamıyor.',
    disconnected: 'Mumble sunucusuyla bağlantı kesildi.',
    invalidCertificate: 'Kayıtlı sertifika geçersiz. Ayarlardan sıfırlayabilirsiniz.',
    kicked: (reason: string) => (reason ? `Sunucudan atıldınız: ${reason}` : 'Sunucudan atıldınız.'),
    banned: (reason: string) => (reason ? `Sunucudan yasaklandınız: ${reason}` : 'Sunucudan yasaklandınız.'),
    // Mumble Reject types
    reject: {
      None: 'Bağlantı reddedildi.',
      WrongVersion: 'Sunucu bu istemci sürümünü kabul etmiyor.',
      InvalidUsername: 'Geçersiz takma ad. Başka bir ad deneyin.',
      WrongUserPW: 'Bu takma ad kayıtlı; şifre veya sertifika yanlış.',
      WrongServerPW: 'Sunucu şifresi yanlış.',
      UsernameInUse: 'Bu takma ad şu anda kullanımda.',
      ServerFull: 'Sunucu dolu.',
      NoCertificate: 'Sunucu sertifika istiyor.',
      AuthenticatorFail: 'Kimlik doğrulama başarısız.',
    } as Record<string, string>,
    // Mumble PermissionDenied types
    denied: {
      Text: 'İzin verilmedi.',
      Permission: 'Bu işlem için yetkiniz yok.',
      SuperUser: 'SuperUser için izin verilmez.',
      ChannelName: 'Geçersiz kanal adı.',
      TextTooLong: 'Mesaj çok uzun.',
      H9K: 'İzin verilmedi.',
      TemporaryChannel: 'Geçici kanalda izin verilmez.',
      MissingCertificate: 'Bu işlem için sertifika gerekli.',
      UserName: 'Geçersiz kullanıcı adı.',
      ChannelFull: 'Kanal dolu.',
      NestingLimit: 'Kanal iç içe geçme sınırına ulaşıldı.',
      ChannelCountLimit: 'Kanal sayısı sınırına ulaşıldı.',
      ChannelListenerLimit: 'Kanal dinleme sınırına ulaşıldı.',
      UserListenerLimit: 'Kullanıcı dinleme sınırına ulaşıldı.',
    } as Record<string, string>,
  },
}
