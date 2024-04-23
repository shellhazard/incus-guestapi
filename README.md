# incus-guestapi

[![ Godoc Reference ][pkg.go.dev_img  ]][pkg.go.dev  ]

A tiny package for communicating with the [Incus instance API](https://linuxcontainers.org/incus/docs/main/dev-incus/#id2).

It has a single dependency (nhooyr.io/websocket) to handle real time events. You can use this package directly or as a reference for your own programs.

## Install

```
go get github.com/shellhazard/incus-guestapi@latest
```

## Purpose

By far the most practical reason to use this is for managing application config. You can pull config values on startup then use `ListenForEvents` to monitor for any changes to the instance configuration in real time - see the example below.

## Usage example

Run the below program inside an Incus instance, then try:
```sh
incus config set instance-name user.secret_config_key=secretvalue
```

```go
package main

import (
	"context"
	"log"

	guest "github.com/shellhazard/incus-guestapi"
	"github.com/shellhazard/incus-guestapi/incus"
)

func must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}
	return t
}

func main() {
	// Make sure we're actually able to use the Incus socket
	if !guest.IsInsideInstance() {
		log.Fatal("failed: not running inside an Incus instance")
	}

	// Create a new API client
	c := guest.NewClient()

	// Retrieve instance state
	info := must(c.Info())
	log.Printf("%+v\n", info)

	// List attached devices
	devices := must(c.Devices())
	log.Printf("%+v\n", devices)

	// List available config keys
	keys := must(c.ListConfig())
	log.Printf("%+v\n", keys)

	// Retrieve the value associated with a key
	value := c.MustConfig("my_config_item")
	log.Printf("my_config_item: %s\n", value)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Block listening for config changes, logging them out as they come
	err := c.ListenForEvents(ctx, func(ev *incus.Event) {
		// Take some kind of useful action here, like updating
		// the config struct used by your application.
		log.Printf("key %s updated - old value: %s; new value: %s\n",
			ev.Config.Key,
			ev.Config.OldValue,
			ev.Config.Value,
		)
	}, incus.EventTypeConfig)
	if err != nil {
		log.Printf("unexpected error in event listener %s\n", err)
	}
}
```

## API Support

The API surface is pretty small. That said, I didn't implement anything I didn't see myself using.
- [x] Instance info (`/1.0`)
- [x] List instance config keys (`/1.0/config`)
- [x] Retrieve config value (`/1.0/config/{key}`)
- [x] List instance devices (`/1.0`)
- [x] Instance info (`/1.0/devices`)
- [x] Retrieve cloud-init metadata (`/1.0/meta-data`)
- [ ] Export images (`/1.0/images/{fingerprint}/export`) (requires `security.guestapi.images` to be set to `true`)