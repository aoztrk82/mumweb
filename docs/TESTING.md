# Manual acceptance tests

**English** | [Türkçe](tr/TESTING.md)

Automated tests (`make test`) are described in the [README](../README.md#testing). This checklist covers what only
a human with real devices can verify. Run it with a desktop Mumble client connected to the same server:

1. Desktop Chrome: log in; the user appears in the desktop Mumble client.
2. Reload the page; you return with the same identity (with the "registered" icon if registered).
3. Switch channels; it changes on the desktop too. Move the user from the desktop; the browser updates.
4. Chat works both ways; a message containing `<script>` does nothing.
5. Speak from the desktop; it is audible in the browser. Speak from the browser with push-to-talk; it is audible on
   the desktop, and the desktop speaking indicator goes out immediately when you release the key.
6. In voice-activation mode in a quiet room no packets are sent (the desktop speaking indicator stays off).
7. Mute and deafen show correctly with the right icons on both sides.
8. Per-user volume only affects the local browser.
9. Turn Wi-Fi off for 10 seconds and back on; the client reconnects automatically.
10. A wrong server password gives a clear error message.
11. Repeat 1–5 in desktop Firefox and Android Chrome; on Android, add to the home screen, check that it opens
    full-screen and that push-to-talk works.
12. With 10 concurrent browser sessions, gateway CPU and RAM stay reasonable (measure with `docker stats`).
