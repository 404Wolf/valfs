# Vals Directory

The valfs submodule has operations for ALL types of vals, including val town
projects and regular val town vals. Some operations can be shared, some cannot.

## `VTFile`s

A `VTFile` is a generic interface for operations for a val-like file. Projects
do not use literal `valgo.ExtendedVal` (the API's schema) vals, but have their
own type that is a bit more generic. Regardless, if a project file is a val, we
still implement a struct compatible with `VTFile`.

A `valfile` is anything that conforms to `./vtfile.go`' `VTFile` interface.

It is sort of like an ORM. Creation depends on the flavor, but in general you
can create a new one using methods like
- `<Regular|Project>VtFileOf`: Create a new *flavored* `VTFile` from an existing val (by,
    say, providing the ID of the val to the method)
- `Create<Regular|Project>ValVtFile`: Create a new *flavored* `VTFile` by
    making a brand new val with the API.

To mutate, you use the setters, and then call `VTFile.Save(ctx)`. To update the
state based on the server state, you can call `VTFile.Load(ctx)`.

### Fusing

When constructing the fuse file system, we create
`<Regular|Project>VTFileInode`s. These take a `VTFile` (of any flavor, as long
as it conforms to the `VTFile` interface!), and expose the functionality
through fuse magic. The `<Regular|Project>VTFileInode`s embed a generic
`VTFileInode` basis.

