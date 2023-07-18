{ pkgs ? (import <nixpkgs> {} )
}:

let

lapki-flasher = pkgs.callPackage ./package.nix {
  # buildGoModule = pkgs.buildGo116Module;
};

in lapki-flasher
