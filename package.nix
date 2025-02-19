{buildGoModule, ...}:
buildGoModule {
  name = "valfs";
  vendorHash = "sha256-0cuwn1jR3pAtgDUdX6oGGJJIWX5BASbQyo5YSKLNkQs=";
  doCheck = false;
  src = ./.;
}
