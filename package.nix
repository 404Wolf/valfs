{buildGoModule, ...}:
buildGoModule {
  name = "valfs";
  vendorHash = "sha256-DNzbPL4R8kIJ+DXIqgBTTNCDars9RWWKBc6HNICbM3w";
  doCheck = false;
  src = ./.;
}
