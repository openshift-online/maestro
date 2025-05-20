package dispatcher

import (
	"testing"

	"github.com/buraksezer/consistent"
	"github.com/google/uuid"

	"github.com/openshift-online/maestro/pkg/api"
)

func TestHashDispatcher(t *testing.T) {
	consistent := consistent.New(nil, consistent.Config{
		PartitionCount:    7,
		ReplicationFactor: 20,
		Load:              1.5,
		Hasher:            hasher{},
	})
	for _, member := range []string{"maestro-maestro-598fb77bf4-rht4s", "maestro-maestro-598fb77bf4-2fslb"} {
		consistent.Add(&api.ServerInstance{
			Meta: api.Meta{
				ID: member,
			},
		})
	}

	var consumers []string
	for i := 0; i < 100; i++ {
		id := uuid.New().String()
		consumers = append(consumers, id)
	}

	for _, consumer := range consumers {
		instance := consistent.LocateKey([]byte(consumer)).String()
		if instance == "" {
			t.Fatalf("should locate to one instance for the consumer %s", consumer)
		}
	}
	consistent.Add(&api.ServerInstance{
		Meta: api.Meta{
			ID: "maestro-maestro-598fb77bf4-b4znx",
		},
	})

	for _, consumer := range consumers {
		instance := consistent.LocateKey([]byte(consumer)).String()
		if instance == "" {
			t.Fatalf("should locate to one instance for the consumer %s", consumer)
		}
	}

	consistent.Remove("maestro-maestro-598fb77bf4-rht4s")

	for _, consumer := range consumers {
		instance := consistent.LocateKey([]byte(consumer)).String()
		if instance == "" {
			t.Fatalf("should locate to one instance for the consumer %s", consumer)
		}
	}
}
