{ pkgs ? ( import <nixpkgs> {} )
}:

let
  name = "lapki-flasher-shell";
in 
pkgs.mkShell {
  inherit name;

  nativeBuildInputs = with pkgs; [
    gcc pkg-config gnumake
    go gopls gopkgs go-tools

    avrdude systemd libusb1
  ];

  shellHook = ''
    echo 'Entering ${name}'
    export GOPATH="$(pwd)/.go"
    export GOCACHE=""
    export GO120MODULE='on'
    cd "$(pwd)/src"
  '';
}