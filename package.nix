{buildGoModule, ...}:
buildGoModule {
  name = "valfs";
  vendorHash = "sha256-KP43QJ/GmvTic+Yv3LLCCO+yBDmdaWpgzGcnYq8VzYw=";
  src = ./.;
}
