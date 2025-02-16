{buildGoModule, ...}:
buildGoModule {
  name = "valfs";
  vendorHash = "sha256-OJmgQLlqBKygITieZg35hXF5snEvltimx6QvFMbN1yM=";
  doCheck = false;
  src = ./.;
}
