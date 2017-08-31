// Copyright 2017 Capsule8 Inc. All rights reserved.

package health

import (
	"errors"
	"net/http"
)

// ConnectionURLs are paths to resources that must be accessable
// for a health check to be possitive
type ConnectionURL string

// Healthy check for all connections that require a 200 response
func (c ConnectionURL) Healthy() error {
	resp, err := http.Get(string(c))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return errors.New(resp.Status)
	}
	return nil
}
