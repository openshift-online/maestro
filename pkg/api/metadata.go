/*
Copyright (c) 2018 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// This file contains functions that simplify generation of API metadata.

package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/golang/glog"
)

// SendAPI sends API documentation response.
func SendAPI(w http.ResponseWriter, r *http.Request) {
	// Set the content type:
	w.Header().Set("Content-Type", "application/json")

	// Prepare the body:
	versions := []VersionMetadata{
		{
			ID:   "v1",
			HREF: r.URL.Path + "/v1",
		},
	}
	body := Metadata{
		ID:       "maestro",
		Kind:     "API",
		HREF:     r.URL.Path,
		Versions: versions,
	}
	data, err := json.Marshal(body)
	if err != nil {
		SendPanic(w, r)
		return
	}

	// Send the response:
	_, err = w.Write(data)
	if err != nil {
		err = fmt.Errorf("Can't send response body for request '%s'", r.URL.Path)
		glog.Error(err)
		sentry.CaptureException(err)
		return
	}
}

// SendAPIV1 sends API version v1 documentation response.
func SendAPIV1(w http.ResponseWriter, r *http.Request) {
	// Set the content type:
	w.Header().Set("Content-Type", "application/json")

	// Prepare the body:
	id := "v1"
	collections := []CollectionMetadata{
		{
			ID:   "resources",
			Kind: "ResourceList",
			HREF: r.URL.Path + "/resources",
		},
	}
	body := VersionMetadata{
		ID:          id,
		Kind:        "APIVersion",
		HREF:        r.URL.Path,
		Collections: collections,
	}
	data, err := json.Marshal(body)
	if err != nil {
		SendPanic(w, r)
		return
	}

	// Send the response:
	_, err = w.Write(data)
	if err != nil {
		glog.Errorf("Can't send response body for request '%s'", r.URL.Path)
		return
	}
}
