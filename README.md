# Val Town File System

## Mount your Val Town vals to a folder, and edit them as if they were just files on your computer.

Quick start
```
export VALTOWN_API_KEY=<something>
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

You can view all the metadata about the val (and the URL it is deployed to!), and some of it you can edit. You can, of course, edit the actual val's content as well.

A `deno.json` file is automatically populated in your vals' directory, so you can edit any of your vals in an IDE that supports the `denols` language server with ease.

## Work in Progress

Disclaimer: this is still a work in progress! Soon, I will...
- Add execute support (in progress) so you can do ./myvals/foo.tsx and it runs on val town's runtime and pipes logs to stdout (this will require a bit of "reverse engineering" the API since it's internal)

# TODOs

Some of the TODOs hinge on Val Town improving their API and exposing more functionality

## Improvements
- add a queue for operations (e.g. deleting) to prevent API limits
- improve .env loading
- add logging
- metadata at top
- trash (to view previous versions)

## ValFile Operations
- rename
- move (out of folder)

## ValFS Operations
- deno.json automatic caching

## Bugs
- creating vals when vals already exist with that name causes them to have an automatic name (maybe this is ok)
