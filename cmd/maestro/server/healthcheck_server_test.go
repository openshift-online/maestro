package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	cegeneric "open-cluster-management.io/sdk-go/pkg/cloudevents/generic"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/db"
)

type mockInstanceDao struct {
	instance *api.ServerInstance
	getErr   error
	replaced *api.ServerInstance
}

func (m *mockInstanceDao) Get(ctx context.Context, id string) (*api.ServerInstance, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.instance, nil
}

func (m *mockInstanceDao) Create(ctx context.Context, instance *api.ServerInstance) (*api.ServerInstance, error) {
	m.instance = instance
	return instance, nil
}

func (m *mockInstanceDao) Replace(ctx context.Context, instance *api.ServerInstance) (*api.ServerInstance, error) {
	m.replaced = instance
	m.instance = instance
	return instance, nil
}

func (m *mockInstanceDao) MarkReadyByIDs(ctx context.Context, ids []string) error   { return nil }
func (m *mockInstanceDao) MarkUnreadyByIDs(ctx context.Context, ids []string) error { return nil }
func (m *mockInstanceDao) Delete(ctx context.Context, id string) error              { return nil }
func (m *mockInstanceDao) DeleteByIDs(ctx context.Context, ids []string) error      { return nil }
func (m *mockInstanceDao) FindByIDs(ctx context.Context, ids []string) (api.ServerInstanceList, error) {
	return nil, nil
}
func (m *mockInstanceDao) FindByUpdatedTime(ctx context.Context, updatedTime time.Time) (api.ServerInstanceList, error) {
	return nil, nil
}
func (m *mockInstanceDao) FindReadyIDs(ctx context.Context) ([]string, error) { return nil, nil }
func (m *mockInstanceDao) All(ctx context.Context) (api.ServerInstanceList, error) {
	if m.instance != nil {
		return api.ServerInstanceList{m.instance}, nil
	}
	return nil, nil
}

type mockLockFactory struct{}

func (m *mockLockFactory) NewAdvisoryLock(ctx context.Context, id string, lockType db.LockType) (string, error) {
	return "test-lock", nil
}
func (m *mockLockFactory) NewNonBlockingLock(ctx context.Context, id string, lockType db.LockType) (string, bool, error) {
	return "test-lock", true, nil
}
func (m *mockLockFactory) Unlock(ctx context.Context, uuid string) {}

type mockCEClient struct {
	ready bool
}

func (m *mockCEClient) OnCreate(ctx context.Context, id string) error { return nil }
func (m *mockCEClient) OnUpdate(ctx context.Context, id string) error { return nil }
func (m *mockCEClient) OnDelete(ctx context.Context, id string) error { return nil }
func (m *mockCEClient) Subscribe(ctx context.Context, handlers ...cegeneric.ResourceHandler[*api.Resource]) {
}
func (m *mockCEClient) Resync(ctx context.Context, consumers []string) error { return nil }
func (m *mockCEClient) SubscribedChan() <-chan struct{}                      { return nil }
func (m *mockCEClient) IsReady() bool                                        { return m.ready }

func TestHealthCheckHandler_CloudEventsReadiness(t *testing.T) {
	origDisable := env().Config.MessageBroker.Disable
	t.Cleanup(func() {
		env().Config.MessageBroker.Disable = origDisable
	})
	env().Config.MessageBroker.Disable = false

	tests := []struct {
		name           string
		instanceReady  bool
		ceClientReady  bool
		hasClient      bool
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "healthy when instance ready and cloudevents ready",
			instanceReady:  true,
			ceClientReady:  true,
			hasClient:      true,
			expectedStatus: http.StatusOK,
			expectedBody:   `{"status": "ok"}`,
		},
		{
			name:           "unhealthy when cloudevents client not ready even if instance ready in DB",
			instanceReady:  true,
			ceClientReady:  false,
			hasClient:      true,
			expectedStatus: http.StatusServiceUnavailable,
			expectedBody:   `{"status": "cloudevents client not ready"}`,
		},
		{
			name:           "unhealthy when instance not ready in DB",
			instanceReady:  false,
			ceClientReady:  true,
			hasClient:      true,
			expectedStatus: http.StatusServiceUnavailable,
			expectedBody:   `{"status": "not ready"}`,
		},
		{
			name:           "healthy when no cloudevents client and instance ready",
			instanceReady:  true,
			hasClient:      false,
			expectedStatus: http.StatusOK,
			expectedBody:   `{"status": "ok"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instanceDao := &mockInstanceDao{
				instance: &api.ServerInstance{
					Meta:          api.Meta{ID: "test-instance"},
					Ready:         tt.instanceReady,
					LastHeartbeat: time.Now(),
				},
			}

			server := &HealthCheckServer{
				instanceDao: instanceDao,
				instanceID:  "test-instance",
			}
			if tt.hasClient {
				server.sourceClient = &mockCEClient{ready: tt.ceClientReady}
			}

			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthcheck", nil)
			rec := httptest.NewRecorder()

			server.healthCheckHandler(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}
			if rec.Body.String() != tt.expectedBody {
				t.Errorf("expected body %q, got %q", tt.expectedBody, rec.Body.String())
			}
		})
	}
}

func TestLivenessHandler(t *testing.T) {
	server := &HealthCheckServer{instanceID: "test-instance"}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/livez", nil)
	rec := httptest.NewRecorder()

	server.livenessHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if rec.Body.String() != `{"status": "ok"}` {
		t.Errorf("expected body %q, got %q", `{"status": "ok"}`, rec.Body.String())
	}
}

func TestPulse_SkipsHeartbeatWhenCloudEventsNotReady(t *testing.T) {
	origDisable := env().Config.MessageBroker.Disable
	t.Cleanup(func() {
		env().Config.MessageBroker.Disable = origDisable
	})
	env().Config.MessageBroker.Disable = false

	initialHeartbeat := time.Now().Add(-10 * time.Minute)
	instanceDao := &mockInstanceDao{
		instance: &api.ServerInstance{
			Meta:          api.Meta{ID: "test-instance"},
			Ready:         true,
			LastHeartbeat: initialHeartbeat,
		},
	}

	server := &HealthCheckServer{
		instanceDao:  instanceDao,
		lockFactory:  &mockLockFactory{},
		instanceID:   "test-instance",
		sourceClient: &mockCEClient{ready: false},
	}

	ctx := context.Background()
	server.pulse(ctx)

	// Heartbeat should NOT be updated because CloudEvents client is not ready
	if instanceDao.replaced != nil {
		t.Errorf("expected pulse to skip heartbeat update when CloudEvents client not ready, but replaced was called")
	}
	if !instanceDao.instance.LastHeartbeat.Equal(initialHeartbeat) {
		t.Errorf("expected LastHeartbeat to remain unchanged, got %v", instanceDao.instance.LastHeartbeat)
	}

	// Now mark CloudEvents client ready and verify pulse updates heartbeat
	server.sourceClient = &mockCEClient{ready: true}
	server.pulse(ctx)

	if instanceDao.replaced == nil {
		t.Errorf("expected pulse to update heartbeat when CloudEvents client is ready")
	}
	if !instanceDao.instance.LastHeartbeat.After(initialHeartbeat) {
		t.Errorf("expected LastHeartbeat to be updated, got %v", instanceDao.instance.LastHeartbeat)
	}
}

func TestCloudEventsNotReady_SkippedWhenMessageBrokerDisabled(t *testing.T) {
	origDisable := env().Config.MessageBroker.Disable
	t.Cleanup(func() {
		env().Config.MessageBroker.Disable = origDisable
	})

	server := &HealthCheckServer{
		instanceID:   "test-instance",
		sourceClient: &mockCEClient{ready: false},
	}

	env().Config.MessageBroker.Disable = true
	if server.cloudEventsNotReady() {
		t.Errorf("expected cloudEventsNotReady to be false when message broker is disabled, even if source client reports not ready")
	}

	env().Config.MessageBroker.Disable = false
	if !server.cloudEventsNotReady() {
		t.Errorf("expected cloudEventsNotReady to be true when message broker is enabled and source client is not ready")
	}
}
