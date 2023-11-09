package util

import (
	"context"
	"fmt"
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

func GetAccountIDFromContext(ctx context.Context) string {
	accountID := ctx.Value("accountID")
	if accountID == nil {
		return ""
	}
	return fmt.Sprintf("%v", accountID)
}
