package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	workv1 "open-cluster-management.io/api/work/v1"

	"github.com/openshift-online/maestro/test/mocks/workserver/requests"
)

// WorkServerClient is a client for interacting with the workserver
type WorkServerClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewWorkServerClient creates a new client
func NewWorkServerClient(baseURL string) *WorkServerClient {
	return &WorkServerClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Create creates a new ManifestWork from a runtime object
func (c *WorkServerClient) Create(ctx context.Context, work *workv1.ManifestWork) (*workv1.ManifestWork, error) {
	// Marshal the work to JSON
	workBytes, err := json.Marshal(work)
	if err != nil {
		return nil, errors.NewBadRequest(fmt.Sprintf("failed to marshal object: %v", err))
	}

	data, err := json.Marshal(requests.CreateRequest{WorkBytes: workBytes})
	if err != nil {
		return nil, errors.NewBadRequest(fmt.Sprintf("failed to marshal request: %v", err))
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/works", bytes.NewBuffer(data))
	if err != nil {
		return nil, errors.NewServiceUnavailable(fmt.Sprintf("failed to create request: %v", err))
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, errors.NewServiceUnavailable(fmt.Sprintf("failed to send request: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, errors.NewInternalError(fmt.Errorf("unexpected response: %s", string(body)))
	}

	var createdWork workv1.ManifestWork
	if err := json.NewDecoder(resp.Body).Decode(&createdWork); err != nil {
		return nil, errors.NewInternalError(fmt.Errorf("failed to decode response: %v", err))
	}

	return &createdWork, nil
}

// Patch patches an existing ManifestWork with raw patch data
func (c *WorkServerClient) Patch(ctx context.Context, name string, patchData []byte) (*workv1.ManifestWork, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPatch, c.baseURL+"/api/v1/works/"+name, bytes.NewBuffer(patchData))
	if err != nil {
		return nil, errors.NewServiceUnavailable(fmt.Sprintf("failed to create request: %v", err))
	}
	httpReq.Header.Set("Content-Type", "application/merge-patch+json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, errors.NewServiceUnavailable(fmt.Sprintf("failed to send request: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, errors.NewInternalError(fmt.Errorf("unexpected response: %s", string(body)))
	}

	var work workv1.ManifestWork
	if err := json.NewDecoder(resp.Body).Decode(&work); err != nil {
		return nil, errors.NewInternalError(fmt.Errorf("failed to decode response: %v", err))
	}

	return &work, nil
}

// Get retrieves a ManifestWork by name via gRPC client
func (c *WorkServerClient) Get(ctx context.Context, name string) (*workv1.ManifestWork, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/works/"+name, nil)
	if err != nil {
		return nil, errors.NewServiceUnavailable(fmt.Sprintf("failed to create request: %v", err))
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, errors.NewServiceUnavailable(fmt.Sprintf("failed to send request: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return nil, errors.NewNotFound(schema.GroupResource{
				Group:    "work.open-cluster-management.io",
				Resource: "manifestworks",
			}, name)
		}

		body, _ := io.ReadAll(resp.Body)
		return nil, errors.NewInternalError(fmt.Errorf("unexpected response: %s", string(body)))
	}

	var work workv1.ManifestWork
	if err := json.NewDecoder(resp.Body).Decode(&work); err != nil {
		return nil, errors.NewInternalError(fmt.Errorf("failed to decode response: %v", err))
	}

	return &work, nil
}
