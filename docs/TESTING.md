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

Admin panel (`MUMWEB_ADMIN_PASSWORD` set; the local environment uses `admin`):

13. `/admin` without the variable set shows "disabled" after login. With it: a wrong admin password is refused, and
    after 5 wrong guesses the IP is locked out for a while while the main login still works.
14. Log in to `/admin/` as `SuperUser` while the main client is connected with a normal nickname in another tab of the
    same browser: both stay connected (no one is kicked) and the panel lists the voice user under **Online**. Speak in
    the voice tab; audio works as before.
15. **Channels:** create, rename and remove a channel; set a channel password and check a normal user is asked for it;
    open **ACL and groups**, tick a permission, add a group member, save, reopen and see the change. On a desktop
    client the same ACL shows in the ACL editor.
16. **Bans:** ban a user from **Online** (certificate only), see the entry, remove it and save; the user can log in
    again. A ban made from the desktop client (IP + hash) can be removed here.
17. **Registered users:** register a user from **Online**, rename it and delete it in the panel. Do ten quick actions in
    a row and check none is silently lost (a "too many actions" message may appear instead).

Security checks (see [SECURITY.md](SECURITY.md)):

18. With the browser's developer tools open, use the voice client and every admin tab: the console shows no
    Content-Security-Policy violation. `curl -sI https://your-domain/` lists the CSP, `X-Frame-Options`,
    `Referrer-Policy` and `Strict-Transport-Security` headers.
19. `docker exec <gateway container> wget -T 4 -O- http://1.1.1.1` fails with "Network unreachable", and
    `docker exec <gateway container> touch /x` fails (read-only).
20. Open more than `MUMWEB_MAX_SESSIONS_PER_IP` tabs from one computer: the extra ones are refused ("too many
    sessions") while the first ones keep working.
