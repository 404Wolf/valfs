# Val Town File System
## Mount your val town vals as a FUSE file system to interact with vals as if they were just typescript files.

Quick start (nix)
```
export VALTOWN_API_KEY=<something>
mkdir ./myvals && nix run github:404wolf/valfs -- mount ./myvals
```

## Work in Progress

Disclaimer: this is still a work in progress! Soon, I will...
- Rewrite the FUSE stuff with [cgofuse](https://github.com/winfsp/cgofuse) to make this cross-platform
- Add support for edit/delete operations
- Add execute support (in progress) so you can do ./myvals/foo.tsx and it runs on val town's runtime and pipes logs to stdout (this will require a bit of "reverse engineering" the API since it's internal)
- Add `-d` mode to create directories for each val, with its settings (e.g. visibility) and README

