package module

import (
	"bytes"
	"fmt"
	"golang.org/x/mod/sumdb"
	"io"
	"net/url"
)

// An in-memory implementation of SumDbClient
type memoryClient struct {
	base  *url.URL
	state chan memoryClientState
}

// In-memory config and cache data stores
type memoryClientState struct {
	config map[string][]byte
	cache  map[string][]byte
}

// Returns a new SubDbClient for the given url
func NewClient(url *url.URL) *sumdb.Client {
	client := &memoryClient{
		base:  url,
		state: make(chan memoryClientState, 1),
	}
	state := memoryClientState{
		config: make(map[string][]byte),
		cache:  make(map[string][]byte),
	}

	// Currently there's only one public sumdb
	state.config["key"] = []byte("sum.golang.org+033de0ae+Ac4zctda0e5eza+HJyk9SxEdh+s3Ux18htTTAD8OuAn8")
	client.state <- state

	return sumdb.NewClient(client)
}

func (c *memoryClient) ReadRemote(path string) ([]byte, error) {
	sumUrl, err := c.base.Parse(path)
	if err != nil {
		return []byte{}, err
	}

	dataResp, err := HttpGet(sumUrl)
	if err != nil {
		return []byte{}, err
	}

	return io.ReadAll(dataResp)
}

func (c *memoryClient) ReadConfig(file string) (data []byte, err error) {
	state := <-c.state
	defer func() { c.state <- state }()

	value, present := state.config[file]
	if !present {
		return []byte{}, nil
	}

	return value, nil
}

func (c *memoryClient) WriteConfig(file string, oldValue, value []byte) error {
	state := <-c.state
	defer func() { c.state <- state }()

	current, present := state.config[file]
	if present && !bytes.Equal(current, oldValue) {
		return sumdb.ErrWriteConflict
	}

	state.config[file] = value
	return nil
}

func (c *memoryClient) ReadCache(file string) ([]byte, error) {
	state := <-c.state
	defer func() { c.state <- state }()

	value, present := state.cache[file]
	if !present {
		return nil, fmt.Errorf("Not exist")
	}
	return value, nil
}

func (c *memoryClient) WriteCache(file string, value []byte) {
	state := <-c.state
	state.cache[file] = value
	c.state <- state
}

func (*memoryClient) Log(msg string) {
}

func (*memoryClient) SecurityError(msg string) {
	fmt.Println(msg)
}
