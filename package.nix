{buildGoModule, ...}:
buildGoModule {
  name = "valfs";
  vendorHash = "sha256-4QNj+eFxEV3fciBB1GUA5vTCx+ElrwvQWHMH8qzOCoE=";
  doCheck = false;
  src = ./.;
}
