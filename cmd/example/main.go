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
