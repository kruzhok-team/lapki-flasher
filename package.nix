{ stdenv, lib, buildGoModule
, makeWrapper, pkg-config, libusb1, avrdude
}:

buildGoModule rec {
  pname = "lapki-flasher";
  version = "0.1";

  src = ../flasher/src;
  
  vendorSha256 = "sha256-LCZ3iV8cAzlCGvqFxWmYKD47tyg12RTCGUOwX89K2EU="; 

  nativeBuildInputs = [ pkg-config makeWrapper ];
  propagatedBuildInputs = [ libusb1 avrdude ];

  subPackages = ["."];

  postInstall = ''
    wrapProgram $out/bin/lapki-flasher \
    --set PATH /bin:${lib.makeBinPath [avrdude]}
  '';
}


