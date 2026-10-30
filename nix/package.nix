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
      ../ui.go
      ../clipboard.go
      ../internal
    ];
  };

  vendorHash = "sha256-SBH2f1kB27QGFoIK6a8QQDyUMoLdntlDCu+eDWEHbyU=";

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
