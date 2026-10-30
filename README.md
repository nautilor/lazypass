# lazypass

Lazygit-style TUI password manager (Go + [gocui](https://github.com/jesseduffield/gocui)).

- Encrypts with your `~/.ssh/id_rsa` (RSA only; prompts for passphrase if the key has one).
- Each workspace (e.g. `personal`, `work`) is a separate file in `~/.config/lazypass/`, with its own random AES-256-GCM data key, wrapped using RSA-OAEP. Entry names are encrypted too.
- Clipboard via `wl-copy`/`xclip`/`xsel`/`pbcopy`, auto-cleared after 20s.

```
go build -o lazypass . && ./lazypass            # flags: -key PATH  -dir PATH ($LAZYPASS_DIR)
```

Press `?` in-app for keys. Lose the private key = lose the passwords.

## Nix

```
nix run github:<you>/lazypass     # or: nix run .
nix develop                       # dev shell: go, gopls, delve, wl-clipboard
```

NixOS:

```nix
{
  inputs.lazypass.url = "github:<you>/lazypass";
  # in your system config:
  imports = [ inputs.lazypass.nixosModules.default ];
  programs.lazypass.enable = true;
}
```

After changing Go dependencies, update `vendorHash` in `nix/package.nix`.

## Config

`~/.config/lazypass/config.yml` (or `-config PATH`) customises the look, using lazygit's
`gui.theme` / `gui.border` keys. See `config.example.yml`.
