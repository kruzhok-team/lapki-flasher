{ stdenv, lib, buildGoModule
, makeWrapper, pkg-config
, libusb1, avrdude, systemd
}:

buildGoModule rec {
  pname = "lapki-flasher";
  version = "0.1";

  src = ../flasher/src;
  
  vendorSha256 = "sha256-sWCi7ZfBAH8xYZukxyFFa08MBb+xWG5r1nmP7IZuBGE="; 

  nativeBuildInputs = [ pkg-config makeWrapper ];
  propagatedBuildInputs = [ libusb1 avrdude systemd ];

  subPackages = ["."];

  postInstall = ''
    wrapProgram $out/bin/lapki-flasher \
    --set PATH /bin:${lib.makeBinPath [ avrdude systemd ]}
  '';
}


