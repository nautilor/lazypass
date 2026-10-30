# lazypass

Lazygit-style TUI password manager (Go + [gocui](https://github.com/jesseduffield/gocui)).

- Encrypts with your `~/.ssh/id_rsa` (RSA only; prompts for passphrase if the key has one).
- Each workspace (e.g. `personal`, `work`) is a separate file in `~/.config/lazypass/`, with its own random AES-256-GCM data key, wrapped using RSA-OAEP. Entry names are encrypted too.
- Clipboard via `wl-copy`/`xclip`/`xsel`/`pbcopy`, auto-cleared after 20s.

```
go build -o lazypass . && ./lazypass            # flags: -key PATH  -dir PATH ($LAZYPASS_DIR)
```

Press `?` in-app for keys. Lose the private key = lose the passwords.
