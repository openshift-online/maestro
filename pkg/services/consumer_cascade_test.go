package services

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"open-cluster-management.io/sdk-go/pkg/cloudevents/clients/common"
	sourcecodec "open-cluster-management.io/sdk-go/pkg/cloudevents/clients/work/source/codec"
	cetypes "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/dao/mocks"
	dbmocks "github.com/openshift-online/maestro/pkg/db/mocks"
)

// TestBuildConsumerCascadeEvent verifies the terminal condition and source identity round-trip through the SDK.
func TestBuildConsumerCascadeEvent(t *testing.T) {
	resource := &api.Resource{Meta: api.Meta{ID: "bundle-id"}, ConsumerName: "consumer", Source: "test-source", Version: 7}
	spec := cloudevents.NewEvent()
	spec.SetID("spec-id")
	spec.SetSource(resource.Source)
	spec.SetType("io.open-cluster-management.works.v1alpha1.manifestbundles.spec.create_request")
	spec.SetExtension(cetypes.ExtensionWorkMeta, `{"name":"root-work","namespace":"consumer","uid":"root-uid"}`)
	if err := spec.SetData(cloudevents.ApplicationJSON, map[string]interface{}{"manifests": []interface{}{}}); err != nil {
		t.Fatal(err)
	}
	var err error
	resource.Payload, err = api.CloudEventToJSONMap(&spec)
	if err != nil {
		t.Fatal(err)
	}
	event, err := buildConsumerCascadeEvent(resource)
	if err != nil {
		t.Fatal(err)
	}
	if event.ResourceID != resource.ID || event.ResourceSource != resource.Source || event.StatusEventType != api.StatusDeleteEventType {
		t.Fatalf("incorrect terminal event: %+v", event)
	}
	status, err := api.JSONMAPToCloudEvent(event.Status)
	if err != nil {
		t.Fatal(err)
	}
	work, err := sourcecodec.NewManifestBundleCodec().Decode(status)
	if err != nil {
		t.Fatalf("synthetic event must decode through the real source codec: %v", err)
	}
	condition := meta.FindStatusCondition(work.Status.Conditions, common.ResourceDeleted)
	if condition == nil || condition.Status != metav1.ConditionTrue || condition.Reason != ConsumerCascadeReason {
		t.Fatalf("missing explicit-cascade deletion condition: %+v", condition)
	}
	if work.Name != "root-work" || work.Namespace != "consumer" || string(work.UID) != resource.ID || work.Generation != 7 {
		t.Fatalf("source work identity lost: %+v", work.ObjectMeta)
	}
}

type consumerCascadeFunc func(context.Context, string, time.Duration, dao.ConsumerCascadeEventBuilder) (api.ResourceList, error)

// Delete delegates to the test callback to inject cascade results and commit failures.
func (f consumerCascadeFunc) Delete(ctx context.Context, id string, threshold time.Duration,
	build dao.ConsumerCascadeEventBuilder) (api.ResourceList, error) {
	return f(ctx, id, threshold, build)
}

// TestConsumerCascadeService checks error translation and committed-only resource accounting.
func TestConsumerCascadeService(t *testing.T) {
	for _, tc := range []struct {
		name      string
		err       error
		wantCode  int
		wantCount float64
	}{
		{"success", nil, 0, 1},
		{"live or recent resource", dao.ErrConsumerCascadeBlocked, 403, 0},
		{"rolled back transaction", errors.New("commit failed"), 500, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			consumerDao := mocks.NewConsumerDao()
			_, err := consumerDao.Create(context.Background(), &api.Consumer{Meta: api.Meta{ID: "id"}, Name: "consumer"})
			if err != nil {
				t.Fatal(err)
			}
			cascade := consumerCascadeFunc(func(ctx context.Context, id string, threshold time.Duration,
				build dao.ConsumerCascadeEventBuilder) (api.ResourceList, error) {
				if id != "id" || threshold != time.Hour || build == nil {
					t.Fatal("incorrect cascade inputs")
				}
				return api.ResourceList{&api.Resource{Meta: api.Meta{ID: "resource"}}}, tc.err
			})
			service := NewConsumerServiceWithCascade(consumerDao, cascade, dbmocks.NewMockAdvisoryLockFactory(), 3600)
			before := testutil.ToFloat64(consumerCascadedResourcesTotal)
			svcErr := service.Delete(context.Background(), "id")
			if tc.wantCode == 0 && svcErr != nil {
				t.Fatal(svcErr)
			}
			if tc.wantCode != 0 && (svcErr == nil || svcErr.HttpCode != tc.wantCode) {
				t.Fatalf("service error = %v, want HTTP %d", svcErr, tc.wantCode)
			}
			if count := testutil.ToFloat64(consumerCascadedResourcesTotal) - before; count != tc.wantCount {
				t.Fatalf("committed resource count = %v, want %v", count, tc.wantCount)
			}
		})
	}
}

// TestConsumerCascadeInvalidThresholdFailsClosed ensures invalid inputs cannot reach the destructive DAO.
func TestConsumerCascadeInvalidThresholdFailsClosed(t *testing.T) {
	for _, threshold := range []int{0, -1, math.MaxInt, int(math.MaxInt64/int64(time.Second))*2 + 100} {
		cascade := consumerCascadeFunc(func(context.Context, string, time.Duration, dao.ConsumerCascadeEventBuilder) (api.ResourceList, error) {
			t.Fatal("invalid threshold must never reach the cascade DAO")
			return nil, nil
		})
		service := NewConsumerServiceWithCascade(nil, cascade, nil, threshold)
		if err := service.Delete(context.Background(), "consumer"); err == nil || err.HttpCode != 403 {
			t.Fatalf("threshold %d: got %v, want forbidden", threshold, err)
		}
	}
}

// TestConsumerCascadeRejectsUndecodableStatus prevents finalizing resources whose watchers could not decode deletion.
func TestConsumerCascadeRejectsUndecodableStatus(t *testing.T) {
	for _, tc := range []struct {
		name     string
		metadata string
		data     interface{}
	}{
		{"invalid spec data", `{}`, "not a manifest bundle"},
		{"invalid work metadata", `{"name":12}`, map[string]interface{}{"manifests": []interface{}{}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec := cloudevents.NewEvent()
			spec.SetID("id")
			spec.SetSource("source")
			spec.SetType("spec")
			spec.SetExtension(cetypes.ExtensionWorkMeta, tc.metadata)
			if err := spec.SetData(cloudevents.ApplicationJSON, tc.data); err != nil {
				t.Fatal(err)
			}
			payload, err := api.CloudEventToJSONMap(&spec)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := buildConsumerCascadeEvent(&api.Resource{Meta: api.Meta{ID: "resource"}, Version: 1, Payload: payload}); err == nil {
				t.Fatal("an undecodable source event must refuse finalization")
			}
		})
	}
}
