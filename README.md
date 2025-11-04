# sciplayer API

## Requirements
- Go 1.25.3 or later
- SQLite (the driver uses `github.com/mattn/go-sqlite3`, which requires a CGO toolchain)

## Getting started
```fish
cd /home/eduardo/code/sciplayer/sciplayer-api
go run ./cmd/sciplayer-api
```

The server listens on `:8090` by default and uses `data/sciplayer.db` as its backing store. Override these with the `SCIPLAYER_HTTP_ADDR` and `SCIPLAYER_DB_PATH` environment variables if required. The database file and parent directory are created automatically if they do not exist.

## API overview

### Register a device
```
POST /devices
{
	"deviceId": "device-123"
}
```

### Attach a playlist to a device
```
POST /devices/{deviceId}/playlists
{
	"name": "My playlist",
	"url": "https://example.com/channel.m3u8"
}
```

### Fetch playlists for a device
```
GET /devices/{deviceId}/playlists
```

### Health probe
```
GET /healthz
```

All responses are JSON encoded. Errors return an object with an `error` field describing the failure.
