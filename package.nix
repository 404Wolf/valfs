{buildGoModule, ...}:
buildGoModule {
  name = "valfs";
  vendorHash = "sha256-ppj9qAv/+1YBvFYtFSV0eNw5OzQa68IOUmSzO2ZewWI=";
  doCheck = false;
  src = ./.;
}
