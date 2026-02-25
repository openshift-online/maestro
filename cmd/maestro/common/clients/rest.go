package clients

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/openshift-online/maestro/pkg/api/openapi"
)

// RESTClient wraps the Maestro OpenAPI client
type RESTClient struct {
	client *openapi.APIClient
	ctx    context.Context
}

// NewRESTClient creates a new REST client from configuration
func NewRESTClient(cfg *RESTConfig) (*RESTClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("REST config is required")
	}

	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("REST base URL is required")
	}

	client := openapi.NewAPIClient(&openapi.Configuration{
		DefaultHeader:    make(map[string]string),
		UserAgent:        "OpenAPI-Generator/1.0.0/go",
		Debug:            false,
		Servers:          openapi.ServerConfigurations{{URL: cfg.BaseURL}},
		OperationServers: map[string]openapi.ServerConfigurations{},
		HTTPClient: func() *http.Client {
			tr := http.DefaultTransport.(*http.Transport).Clone()
			tr.TLSClientConfig = &tls.Config{
				MinVersion:         tls.VersionTLS13,
				InsecureSkipVerify: cfg.InsecureSkipVerify,
			}
			return &http.Client{
				Transport: tr,
				Timeout:   cfg.Timeout,
			}
		}(),
	})

	return &RESTClient{
		client: client,
		ctx:    context.Background(),
	}, nil
}

// ListResourceBundles lists resource bundles with pagination and filtering
func (c *RESTClient) ListResourceBundles(ctx context.Context, page, size int, search string) (*openapi.ResourceBundleList, error) {
	req := c.client.DefaultAPI.ApiMaestroV1ResourceBundlesGet(ctx).
		Page(int32(page)).
		Size(int32(size))

	if search != "" {
		req = req.Search(search)
	}

	result, resp, err := req.Execute()
	if resp == nil {
		return nil, fmt.Errorf("no HTTP response received, err=%w", err)
	}

	defer resp.Body.Close()

	// Check status code first (response is available even when err != nil)
	switch resp.StatusCode {
	case http.StatusOK:
		if err != nil {
			return nil, fmt.Errorf("failed to decode resource bundle list response: %w", err)
		}
		return result, nil
	case http.StatusNotFound:
		return nil, fmt.Errorf("resource bundle not found")
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("authentication failed")
	case http.StatusForbidden:
		return nil, fmt.Errorf("permission denied")
	default:
		return nil, fmt.Errorf("unexpected status code %d, err=%w", resp.StatusCode, err)
	}
}

// GetResourceBundle retrieves a single resource bundle by ID
func (c *RESTClient) GetResourceBundle(ctx context.Context, id string) (*openapi.ResourceBundle, error) {
	result, resp, err := c.client.DefaultAPI.ApiMaestroV1ResourceBundlesIdGet(ctx, id).Execute()
	if resp == nil {
		return nil, fmt.Errorf("no HTTP response received, err=%w", err)
	}

	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		if err != nil {
			return nil, fmt.Errorf("failed to decode resource bundle response: %w", err)
		}
		return result, nil
	case http.StatusNotFound:
		return nil, fmt.Errorf("resource bundle not found")
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("authentication failed")
	case http.StatusForbidden:
		return nil, fmt.Errorf("permission denied")
	default:
		return nil, fmt.Errorf("unexpected status code %d, err=%w", resp.StatusCode, err)
	}
}

// DeleteResourceBundle deletes a resource bundle by ID
func (c *RESTClient) DeleteResourceBundle(ctx context.Context, id string) error {
	resp, err := c.client.DefaultAPI.ApiMaestroV1ResourceBundlesIdDelete(ctx, id).Execute()
	if resp == nil {
		return fmt.Errorf("no HTTP response received, err=%w", err)
	}

	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return fmt.Errorf("resource bundle not found")
	case http.StatusUnauthorized:
		return fmt.Errorf("authentication failed")
	case http.StatusForbidden:
		return fmt.Errorf("permission denied")
	default:
		return fmt.Errorf("unexpected status code %d, err=%w", resp.StatusCode, err)
	}
}

// ListConsumers lists consumers with pagination and filtering
func (c *RESTClient) ListConsumers(ctx context.Context, page, size int, search string) (*openapi.ConsumerList, error) {
	req := c.client.DefaultAPI.ApiMaestroV1ConsumersGet(ctx).
		Page(int32(page)).
		Size(int32(size))

	if search != "" {
		req = req.Search(search)
	}

	result, resp, err := req.Execute()
	if resp == nil {
		return nil, fmt.Errorf("no HTTP response received, err=%w", err)
	}

	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		if err != nil {
			return nil, fmt.Errorf("failed to decode consumer list response: %w", err)
		}
		return result, nil
	case http.StatusNotFound:
		return nil, fmt.Errorf("consumer not found")
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("authentication failed")
	case http.StatusForbidden:
		return nil, fmt.Errorf("permission denied")
	default:
		return nil, fmt.Errorf("unexpected status code %d, err=%w", resp.StatusCode, err)
	}

}

// GetConsumer retrieves a single consumer by ID
func (c *RESTClient) GetConsumer(ctx context.Context, id string) (*openapi.Consumer, error) {
	result, resp, err := c.client.DefaultAPI.ApiMaestroV1ConsumersIdGet(ctx, id).Execute()
	if resp == nil {
		return nil, fmt.Errorf("no HTTP response received, err=%w", err)
	}

	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		if err != nil {
			return nil, fmt.Errorf("failed to decode consumer response: %w", err)
		}
		return result, nil
	case http.StatusNotFound:
		return nil, fmt.Errorf("consumer not found")
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("authentication failed")
	case http.StatusForbidden:
		return nil, fmt.Errorf("permission denied")
	default:
		return nil, fmt.Errorf("unexpected status code %d, err=%w", resp.StatusCode, err)
	}
}

// CreateConsumer creates a new consumer
func (c *RESTClient) CreateConsumer(ctx context.Context, consumer openapi.Consumer) (*openapi.Consumer, error) {
	result, resp, err := c.client.DefaultAPI.ApiMaestroV1ConsumersPost(ctx).Consumer(consumer).Execute()
	if resp == nil {
		return nil, fmt.Errorf("no HTTP response received, err=%w", err)
	}

	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusCreated:
		if err != nil {
			return nil, fmt.Errorf("failed to decode consumer response: %w", err)
		}
		return result, nil
	case http.StatusBadRequest:
		return nil, fmt.Errorf("bad request")
	case http.StatusConflict:
		return nil, fmt.Errorf("consumer already exists")
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("authentication failed")
	case http.StatusForbidden:
		return nil, fmt.Errorf("permission denied")
	default:
		return nil, fmt.Errorf("unexpected status code %d, err=%w", resp.StatusCode, err)
	}
}

// UpdateConsumer updates an existing consumer
func (c *RESTClient) UpdateConsumer(ctx context.Context, id string, consumer openapi.ConsumerPatchRequest) (*openapi.Consumer, error) {
	result, resp, err := c.client.DefaultAPI.ApiMaestroV1ConsumersIdPatch(ctx, id).ConsumerPatchRequest(consumer).Execute()
	if resp == nil {
		return nil, fmt.Errorf("no HTTP response received, err=%w", err)
	}

	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		if err != nil {
			return nil, fmt.Errorf("failed to decode consumer response: %w", err)
		}
		return result, nil
	case http.StatusNotFound:
		return nil, fmt.Errorf("consumer not found")
	case http.StatusBadRequest:
		return nil, fmt.Errorf("bad request")
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("authentication failed")
	case http.StatusForbidden:
		return nil, fmt.Errorf("permission denied")
	default:
		return nil, fmt.Errorf("unexpected status code %d, err=%w", resp.StatusCode, err)
	}
}

// DeleteConsumer deletes a consumer by ID
func (c *RESTClient) DeleteConsumer(ctx context.Context, id string) error {
	resp, err := c.client.DefaultAPI.ApiMaestroV1ConsumersIdDelete(ctx, id).Execute()
	if resp == nil {
		return fmt.Errorf("no HTTP response received, err=%w", err)
	}

	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return fmt.Errorf("consumer not found")
	case http.StatusConflict:
		return fmt.Errorf("conflict - consumer has existing resources")
	case http.StatusUnauthorized:
		return fmt.Errorf("authentication failed")
	case http.StatusForbidden:
		return fmt.Errorf("permission denied")
	default:
		return fmt.Errorf("unexpected status code %d, err=%w", resp.StatusCode, err)
	}
}
