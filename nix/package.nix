{ lib, buildGoModule, makeWrapper, wl-clipboard, xclip, xsel }:

buildGoModule {
  pname = "lazypass";
  version = "0.1.0";

  src = lib.fileset.toSource {
    root = ../.;
    fileset = lib.fileset.unions [
      ../go.mod
      ../go.sum
      ../main.go
      ../internal
    ];
  };

  vendorHash = "sha256-zNGUlQ8iSUZ8rfBLUlgaCef23/+Dm3Jx/P3bVEW/cEg=";

  nativeBuildInputs = [ makeWrapper ];

  # clipboard helpers are looked up on PATH at runtime
  postFixup = ''
    wrapProgram $out/bin/lazypass \
      --suffix PATH : ${lib.makeBinPath [ wl-clipboard xclip xsel ]}
  '';

  meta = {
    description = "Lazygit-style TUI password manager encrypted with your SSH RSA key";
    mainProgram = "lazypass";
    license = lib.licenses.mit;
  };
}
