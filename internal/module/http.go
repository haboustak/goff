package module

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Retrieve a file using an HTTP GET request
func HttpGet(url *url.URL) (io.ReadCloser, error) {
	resp, err := http.DefaultClient.Get(url.String())
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		msg, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("Received %v response for %v", resp.StatusCode, url)
		}
		return nil, fmt.Errorf(string(msg))
	}

	return resp.Body, nil
}
