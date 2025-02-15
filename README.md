# Val Town File System

## Mount your Val Town vals to a folder, and edit them as if they were just files on your computer.

Quick start
```
export VAL_TOWN_API_KEY=<something>
mkdir ./myvals
./valfs -- mount ./myvals
```

Now all your vals should show up as `.tsx` files under `./myvals`. You can:
- Edit them, and when you save new versions will automatically be created
- Delete them (be careful!)

## Example File

```ts
#!/home/wolf/Documents/projects/Active/valfs/valfs

/*---
id: 4f3aff9c-c54b-11ef-b3a1-e6cdfca9ef9f # 🔒
version: 40 # 🔒
privacy: private # (public|private|unlisted)
links:
    valtown: https://www.val.town/v/wolf/test # 🔒
    esmModule: https://esm.town/v/wolf/test?v=40 # 🔒
    deployment: https://wolf-test.web.val.run # 🔒
readme: ""
---*/

console.log("Hello world!")
```

Notice the magic shebang! Coming soon...

You can view all the metadata about the val (and the URL it is deployed to!),
and some of it you can edit. You can, of course, edit the actual val's content
as well.

A `deno.json` file is automatically populated in your vals' directory, so you
can edit any of your vals in an IDE that supports the `denols` language server
with ease.

## Work in Progress

Disclaimer: this is still a work in progress! Soon, I will...

- Add execute support (in progress) so you can do ./myvals/foo.tsx and it runs
on val town's runtime and pipes logs to stdout (this will require a bit of
"reverse engineering" the API since it's internal)

# TODOs

Some of the TODOs hinge on Val Town improving their API and exposing more functionality

## Improvements
- add a queue for operations (e.g. deleting) to prevent API limits
- improve .env loading
- add logging
- metadata at top
- trash (to view previous versions)
- ValFiles should only have one val data reference, not two, or it should be 
  better documented how the lazy loading works.

## ValFile Operations
- rename
- move (out of folder)

## ValFS Operations
- deno.json automatic caching

## Bugs
- creating vals when vals already exist with that name causes them to have an automatic name (maybe this is ok)

## Configuration
Add options for:
- whether files should be executable
- whether to do lazy fetching

### Deno
- When you execute a val with ./, use
  `deno run --lock=path/to/the-real-lock.lock script.X.tsx` and have val town
  generate the lock file (with HTTP val wrapper + `Deno.readTextFile()` on
  lockfile)

## Blobs
- Blobs work using loopback with temp files. I want to automatically clean the temp files on exit
