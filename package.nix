{buildGoModule, ...}:
buildGoModule {
  name = "valfs";
  vendorHash = "sha256-wJ2k99i6yXUT+LV6AKLPaJke4xv/dZt47oKPGeQyWoU=";
  doCheck = false;
  src = ./.;
}
