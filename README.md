# Gon(e)tentapi

A ludicrously simple go frontend which connects directly to a 
[contentapi](https://github.com/randomouscrap98/contentapi) database
and serves (currently) readonly content. Made specifically as 
life support for old contentapi instances. 

May someday allow limited forms of writing (don't count on it)

## Requirements

- Go 1.22
- Ability to build CGO libs
- Target that accepts CGO libs (most do)

## Deploy

```
cd gontentapi
go build
rsync gontentapi you@server.com:/path/to/wherever/
rsync -avz static you@server.com:/path/to/wherever/
```

- Building may take a while because of sqlite cgo build
- Running the executable once will generate a default config. You can edit this after
- Running will also generate data folders for thumbnails/etc
- Make sure ownership of relevant folders is correct, particularly the 
  thumbnails folder.
- The "RootPath" variable should be set if you're not hosting it at the root.
  It is JUST the path, so if you host it at https://github.com/randomouscrap98/gontentapi,
  it would be `/randomouscrap98/gontentapi`
- The executable handles all routing. If you put it behind a reverse proxy, you 
  shouldn't have to do anything fancy like handling / at the end of the path

