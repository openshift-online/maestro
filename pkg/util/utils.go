package util

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/cloudevents/sdk-go/v2/event"
)

func EmptyStringToNil(a string) *string {
	if a == "" {
		return nil
	}
	return &a
}

func NilToEmptyString(a *string) string {
	if a == nil {
		return ""
	}
	return *a
}

func NilToEmptyInt32(a *int32) int32 {
	if a == nil {
		return 0
	}
	return *a
}

func GetAccountIDFromContext(ctx context.Context) string {
	accountID := ctx.Value("accountID")
	if accountID == nil {
		return ""
	}
	return fmt.Sprintf("%v", accountID)
}

func FormatEventContext(ec event.EventContext) string {
	b := strings.Builder{}

	// Format core attributes
	b.WriteString(fmt.Sprintf("id=%s type=%s source=%s", ec.GetID(), ec.GetType(), ec.GetSource()))

	// Format extensions if present
	if len(ec.GetExtensions()) > 0 {
		keys := make([]string, 0, len(ec.GetExtensions()))
		for k := range ec.GetExtensions() {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, key := range keys {
			b.WriteString(fmt.Sprintf(" %s=%v", key, ec.GetExtensions()[key]))
		}
	}

	return b.String()
}
