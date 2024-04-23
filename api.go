package guest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/shellhazard/incus-guestapi/incus"
	"nhooyr.io/websocket"
)

// The API is documented here: https://linuxcontainers.org/incus/docs/main/dev-incus/

const (
	SocketPath = "/dev/incus/sock"

	InstanceInfoPath = "/1.0/1.0"
	ListDevicesPath  = "/1.0/1.0/devices"
	ConfigPath       = "/1.0/1.0/config"
	MetadataPath     = "/1.0/1.0/meta-data"
	EventsPath       = "/1.0/1.0/events"
)

var UnexpectedStatusCode = errors.New("unexpected status code")

// IsIncus attempts to connect to /dev/incus/sock.
func IsInsideInstance() bool {
	addr, err := net.ResolveUnixAddr("unix", SocketPath)
	if err != nil {
		return false
	}

	conn, err := net.DialUnix("unix", nil, addr)
	if err != nil {
		return false
	}

	// Successful. We can close out here.
	conn.Close()

	return true
}

type GuestClient struct {
	c *http.Client
}

func NewClient() *GuestClient {
	return &GuestClient{
		c: &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					dialer := net.Dialer{}
					return dialer.DialContext(ctx, "unix", SocketPath)
				},
			},
		},
	}
}

func handlejson[T any](gapi *GuestClient, path string, target T) (T, error) {
	endpoint, err := url.JoinPath("http://", path)
	if err != nil {
		return target, fmt.Errorf("unexpected error: %w", err)
	}

	resp, err := gapi.c.Get(endpoint)
	if err != nil {
		return target, fmt.Errorf("socket error: %w", err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return target, fmt.Errorf("reader error: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return target, fmt.Errorf("%w: %d", UnexpectedStatusCode, resp.StatusCode)
	}

	err = json.Unmarshal(payload, &target)
	if err != nil {
		return target, fmt.Errorf("unmarshal error: %w", err)
	}

	return target, nil
}

// Info returns information about the API and instance state.
//
// See: https://linuxcontainers.org/incus/docs/main/dev-incus/#id2
func (g *GuestClient) Info() (*incus.InstanceInfo, error) {
	r, err := handlejson[incus.InstanceInfo](g, InstanceInfoPath, incus.InstanceInfo{})
	return &r, err
}

// ListConfig returns a slice of all config keys available to the instance.
//
// See: https://linuxcontainers.org/incus/docs/main/dev-incus/#config
func (g *GuestClient) ListConfig() ([]string, error) {
	s := []string{}
	return handlejson[[]string](g, ConfigPath, s)
}

// Devices returns a map of devices available to the instance.
//
// See: https://linuxcontainers.org/incus/docs/main/dev-incus/#devices
func (g *GuestClient) Devices() (map[string]map[string]string, error) {
	m := make(map[string]map[string]string)
	mp, err := handlejson[map[string]map[string]string](g, ListDevicesPath, m)
	return mp, err
}

// HasConfig checks for the presence of the specified config key.
//
// As instances only have access to user.* and cloud-init.*
// configuration, provided keys will be prefixed with `user.`
// unless explicitly specified.
//
// See: https://linuxcontainers.org/incus/docs/main/dev-incus/#config-key
func (g *GuestClient) HasConfig(key string) (bool, error) {
	formattedKey := key
	if !strings.HasPrefix(key, "cloud-init.") && !strings.HasPrefix(key, "user.") {
		formattedKey = fmt.Sprintf("user.%s", key)
	}
	endpoint, err := url.JoinPath("http://", ConfigPath, formattedKey)
	if err != nil {
		return false, fmt.Errorf("unexpected error: %w", err)
	}

	resp, err := g.c.Head(endpoint)
	if err != nil {
		return false, fmt.Errorf("socket error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	} else if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("%w: %d", UnexpectedStatusCode, resp.StatusCode)
	}

	return true, nil
}

// MustConfig calls Config, panicking if there's any
// error or the key is empty.
func (g *GuestClient) MustConfig(key string) string {
	result, err := g.Config(key)
	if err != nil {
		panic(fmt.Errorf("error loading config key %s: %w", key, err))
	}

	if result == "" {
		panic(fmt.Sprintf("loaded key %s is blank", key))
	}

	return result
}

// Config retrieves the value of the specified instance config key.
//
// As instances only have access to user.* and cloud-init.*
// configuration, provided keys will be prefixed with `user.`
// unless explicitly specified.
//
// See: https://linuxcontainers.org/incus/docs/main/dev-incus/#config-key
func (g *GuestClient) Config(key string) (string, error) {
	formattedKey := key
	if !strings.HasPrefix(key, "cloud-init.") && !strings.HasPrefix(key, "user.") {
		formattedKey = fmt.Sprintf("user.%s", key)
	}
	endpoint, err := url.JoinPath("http://", ConfigPath, formattedKey)
	if err != nil {
		return "", fmt.Errorf("unexpected error: %w", err)
	}

	resp, err := g.c.Get(endpoint)
	if err != nil {
		return "", fmt.Errorf("socket error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	} else if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: %d", UnexpectedStatusCode, resp.StatusCode)
	}

	result, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reader error: %w", err)
	}

	return string(result), nil
}

// Metadata returns the value of the `cloud-init.user-data` config key.
//
// See: https://linuxcontainers.org/incus/docs/main/dev-incus/#meta-data
func (g *GuestClient) Metadata() (string, error) {
	var out string
	endpoint, err := url.JoinPath("http://", MetadataPath)
	if err != nil {
		return out, fmt.Errorf("unexpected error: %w", err)
	}

	resp, err := g.c.Get(endpoint)
	if err != nil {
		return out, fmt.Errorf("socket error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: %d", UnexpectedStatusCode, resp.StatusCode)
	}

	result, err := io.ReadAll(resp.Body)
	if err != nil {
		return out, fmt.Errorf("reader error: %w", err)
	}

	return string(result), nil
}

// ListenForEvents opens a WebSocket connection to the guest events API, blocking
// the current goroutine. It takes a callback function and an optional list of events
// to subscribe to. If no events are provided, it will subscribe to all of them.
//
// See the definition for incus.EventType for valid values.
func (g *GuestClient) ListenForEvents(ctx context.Context, callback func(*incus.Event), events ...incus.EventType) error {
	endpoint, err := url.JoinPath("ws://", EventsPath)
	if err != nil {
		return fmt.Errorf("unexpected error: %w", err)
	}

	// Only subscribe to specific events
	if len(events) > 0 {
		parsed, err := url.Parse(endpoint)
		if err != nil {
			return fmt.Errorf("unexpected error: %w", err)
		}

		strEvents := []string{}
		for _, ev := range events {
			if ev.Valid() {
				strEvents = append(strEvents, string(ev))
			}
		}

		val := url.Values{}
		val.Add("type", strings.Join(strEvents, ","))
		parsed.RawQuery = val.Encode()
		endpoint = parsed.String()
	}

	conn, _, err := websocket.Dial(ctx, endpoint, &websocket.DialOptions{
		HTTPClient: g.c,
	})
	if err != nil {
		return err
	}
	defer conn.CloseNow()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			_, message, err := conn.Reader(ctx)
			if err != nil {
				return fmt.Errorf("error in reader: %w", err)
			}

			ev := &incus.Event{}
			err = json.NewDecoder(message).Decode(ev)
			if err != nil {
				return fmt.Errorf("error in json unmarshaller: %w", err)
			}

			go callback(ev)
		}
	}
}
