{buildGoModule, ...}:
buildGoModule {
  name = "valfs";
  vendorHash = "sha256-BP/djan2TkY2uBkFaGAyZN70LnhEMb3R8oUfGkZuN6I=";
  doCheck = false;
  src = ./.;
}
