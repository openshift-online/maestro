package handlers

import (
	"encoding/json"
	"net/http"
	"reflect"
)

func writeJSONResponse(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	// By default, decide whether or not a cache is usable based on the matching of the JWT
	// For example, this will keep caches from being used in the same browser if two users were to log in back to back
	w.Header().Set("Vary", "Authorization")

	w.WriteHeader(code)

	if payload != nil {
		response, _ := json.Marshal(payload)
		_, _ = w.Write(response)
	}
}

// Prepare a 'list' of non-db-backed resources
func determineListRange(obj interface{}, page int, size int64) (list []interface{}, total int64) {
	items := reflect.ValueOf(obj)
	total = int64(items.Len())
	low := int64(page-1) * size
	high := low + size
	if low < 0 || low >= total || high >= total {
		low = 0
		high = total
	}
	for i := low; i < high; i++ {
		list = append(list, items.Index(int(i)).Interface())
	}

	return list, total
}
