{ pkgs ? (import <nixpkgs> {} )
}:

let

lapki-flasher = pkgs.callPackage ./package.nix {
  # buildGoModule = pkgs.buildGo120Module;
};

in lapki-flasher
