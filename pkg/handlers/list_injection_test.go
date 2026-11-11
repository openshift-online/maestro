package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	"gorm.io/gorm/clause"
	"k8s.io/klog/v2"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/errors"
	"github.com/openshift-online/maestro/pkg/services"
)

var resourceListColumns = []string{
	"id", "created_at", "updated_at", "deleted_at",
	"version", "source", "consumer_name", "type",
	"payload", "status", "name",
}

var consumerListColumns = []string{"id", "created_at", "updated_at", "deleted_at", "name", "labels"}

// recordingGenericDao records every query fragment the service layer tries to
// push into the database. The injection tests use it to prove that a blocked
// request never builds or executes a query.
type recordingGenericDao struct {
	table   string
	columns map[string]struct{}

	whereSQL  []string
	orderedBy []string
	counted   bool
	fetched   bool
}

var _ dao.GenericDao = (*recordingGenericDao)(nil)

func newRecordingGenericDao(table string, columns ...string) *recordingGenericDao {
	columnSet := make(map[string]struct{}, len(columns))
	for _, column := range columns {
		columnSet[column] = struct{}{}
	}
	return &recordingGenericDao{table: table, columns: columnSet}
}

func (d *recordingGenericDao) Fetch(int, int, interface{}) error {
	d.fetched = true
	return nil
}

func (d *recordingGenericDao) GetInstanceDao(context.Context, interface{}) dao.GenericDao { return d }

func (*recordingGenericDao) Preload(string) {}

func (d *recordingGenericDao) OrderBy(field string, descending bool) bool {
	resolved, ok := d.ResolveColumn(field)
	if !ok {
		return false
	}
	direction := "asc"
	if descending {
		direction = "desc"
	}
	d.orderedBy = append(d.orderedBy, resolved+" "+direction)
	return true
}

func (d *recordingGenericDao) ResolveColumn(field string) (string, bool) {
	parts := strings.Split(field, ".")
	if len(parts) == 0 || len(parts) > 2 {
		return "", false
	}
	column := toDBName(parts[len(parts)-1])
	if _, ok := d.columns[column]; !ok {
		return "", false
	}
	if len(parts) == 2 {
		if parts[0] != d.table {
			return "", false
		}
		return parts[0] + "." + column, true
	}
	return column, true
}

// toDBName converts a Go struct field name to its database column name the
// way GORM's naming strategy does, e.g. "CreatedAt" -> "created_at" and
// "ID" -> "id". Names that are already lowercase are returned unchanged.
func toDBName(name string) string {
	if strings.ToLower(name) == name {
		return name
	}
	var b strings.Builder
	runes := []rune(name)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r >= 'A' && r <= 'Z' {
			j := i
			for j < len(runes) && runes[j] >= 'A' && runes[j] <= 'Z' {
				j++
			}
			// in a run like "HTTPCode" the last uppercase letter starts the
			// next word, while a full run like "ID" is a single word
			if j < len(runes) && runes[j] >= 'a' && runes[j] <= 'z' && j-i > 1 {
				j--
			}
			if i > 0 {
				b.WriteByte('_')
			}
			for k := i; k < j; k++ {
				b.WriteRune(runes[k] - 'A' + 'a')
			}
			i = j - 1
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (d *recordingGenericDao) ColumnNames() []string {
	columns := make([]string, 0, len(d.columns))
	for column := range d.columns {
		columns = append(columns, column)
	}
	return columns
}

func (d *recordingGenericDao) Joins(dao.TableRelation) {}

func (d *recordingGenericDao) Group(...clause.Column) {}

func (d *recordingGenericDao) Where(sql string, values []interface{}) {
	d.whereSQL = append(d.whereSQL, sql)
}

func (d *recordingGenericDao) Count(model interface{}, total *int64) {
	d.counted = true
	*total = 0
}

func (*recordingGenericDao) Validate(interface{}) error { return nil }

func (d *recordingGenericDao) GetTableName() string { return d.table }

func (*recordingGenericDao) GetTableRelation(string) (dao.TableRelation, bool) {
	return dao.TableRelation{}, false
}

// delegatingResourceService mirrors sqlResourceService.ListWithArgs by
// delegating to the generic list service, keeping the real validation in the
// request path of the resource-bundles endpoint.
type delegatingResourceService struct {
	services.ResourceService
	generic services.GenericService
}

func (s delegatingResourceService) ListWithArgs(ctx context.Context, username string, args *services.ListArguments, resources *[]api.Resource) (*api.PagingMeta, *errors.ServiceError) {
	return s.generic.List(ctx, username, args, resources)
}

func performConsumersList(d dao.GenericDao, query url.Values) *httptest.ResponseRecorder {
	return performConsumersListWithContext(context.Background(), d, query)
}

func performConsumersListWithContext(ctx context.Context, d dao.GenericDao, query url.Values) *httptest.ResponseRecorder {
	handler := NewConsumerHandler(nil, nil, services.NewGenericService(d))
	request := httptest.NewRequest(http.MethodGet, "/api/maestro/v1/consumers?"+query.Encode(), nil)
	request = request.WithContext(ctx)
	recorder := httptest.NewRecorder()
	handler.List(recorder, request)
	return recorder
}

type captureLogSink struct{ output *strings.Builder }

func (s *captureLogSink) Init(logr.RuntimeInfo) {}
func (s *captureLogSink) Enabled(int) bool      { return true }
func (s *captureLogSink) Info(_ int, message string, values ...any) {
	fmt.Fprintln(s.output, message, values)
}
func (s *captureLogSink) Error(err error, message string, values ...any) {
	fmt.Fprintln(s.output, message, err, values)
}
func (s *captureLogSink) WithValues(...any) logr.LogSink { return s }
func (s *captureLogSink) WithName(string) logr.LogSink   { return s }

// TestListValidationLogsDoNotIncludeUserIdentifiers verifies rejected search
// and orderBy values are not exposed through the HTTP handler's validation logs.
func TestListValidationLogsDoNotIncludeUserIdentifiers(t *testing.T) {
	RegisterTestingT(t)

	for _, test := range []struct {
		name      string
		parameter string
		value     string
	}{
		{name: "search", parameter: "search", value: "sensitive_search_identifier = 'x'"},
		{name: "orderBy", parameter: "orderBy", value: "sensitive_order_identifier desc"},
	} {
		t.Run(test.name, func(t *testing.T) {
			logs := &strings.Builder{}
			ctx := klog.NewContext(context.Background(), logr.New(&captureLogSink{output: logs}))
			d := newRecordingGenericDao("consumers", consumerListColumns...)
			query := url.Values{test.parameter: {test.value}}

			recorder := performConsumersListWithContext(ctx, d, query)

			Expect(recorder.Code).To(Equal(http.StatusBadRequest))
			Expect(logs.String()).To(ContainSubstring("user request error"))
			Expect(logs.String()).ToNot(ContainSubstring(test.value))
		})
	}
}

func performResourceBundlesList(d dao.GenericDao, query url.Values) *httptest.ResponseRecorder {
	generic := services.NewGenericService(d)
	handler := NewResourceBundleHandler(delegatingResourceService{generic: generic}, generic)
	request := httptest.NewRequest(http.MethodGet, "/api/maestro/v1/resource-bundles?"+query.Encode(), nil)
	recorder := httptest.NewRecorder()
	handler.List(recorder, request)
	return recorder
}

// TestListSQLIdentifierInjectionBlocked attempts the ACM-38753 SQL identifier
// injection through the search and orderBy query parameters of both list
// endpoints. Every payload must be rejected with a 400 response before any
// query fragment reaches the data access layer.
func TestListSQLIdentifierInjectionBlocked(t *testing.T) {
	RegisterTestingT(t)

	attackQueries := []url.Values{
		// orderBy: raw SQL expressions must not reach the ORDER BY clause
		{"orderBy": {"(SELECT/**/1)"}},
		{"orderBy": {"lower(name)"}},
		{"orderBy": {`name" OR 1=1 --`}},
		{"orderBy": {"name;DROP TABLE resources"}},
		{"orderBy": {"name desc; DROP TABLE resources"}},
		// search: the properties.* rewrite and quoted-identifier breakouts
		{"search": {`properties.foo = 'bar'`}},
		{"search": {`"name' OR 1=1 --" = 'x'`}},
		{"search": {"`name' OR 1=1 --` = 'x'"}},
		{"search": {"not_a_column = 'x'"}},
		{"search": {"unknown_column -> 'k' ->> 'k2' = 'v'"}},
	}

	endpoints := []struct {
		name    string
		table   string
		columns []string
		perform func(d dao.GenericDao, query url.Values) *httptest.ResponseRecorder
	}{
		{
			name:    "consumers",
			table:   "consumers",
			columns: consumerListColumns,
			perform: performConsumersList,
		},
		{
			name:    "resource-bundles",
			table:   "resources",
			columns: resourceListColumns,
			perform: performResourceBundlesList,
		},
	}

	for _, endpoint := range endpoints {
		for _, query := range attackQueries {
			testDao := newRecordingGenericDao(endpoint.table, endpoint.columns...)

			recorder := endpoint.perform(testDao, query)

			Expect(recorder.Code).To(Equal(http.StatusBadRequest),
				"%s: %s", endpoint.name, query.Encode())
			Expect(recorder.Body.String()).To(ContainSubstring("maestro-"),
				"%s: %s", endpoint.name, query.Encode())
			// the exploit must be rejected before any query is built or executed
			Expect(testDao.orderedBy).To(BeEmpty(), "%s: %s", endpoint.name, query.Encode())
			Expect(testDao.whereSQL).To(BeEmpty(), "%s: %s", endpoint.name, query.Encode())
			Expect(testDao.counted).To(BeFalse(), "%s: %s", endpoint.name, query.Encode())
			Expect(testDao.fetched).To(BeFalse(), "%s: %s", endpoint.name, query.Encode())
		}

		// a valid request against the same endpoint still succeeds
		testDao := newRecordingGenericDao(endpoint.table, endpoint.columns...)

		recorder := endpoint.perform(testDao, url.Values{
			"orderBy": {"name desc"},
			"search":  {"name like 'c%'"},
		})

		Expect(recorder.Code).To(Equal(http.StatusOK), endpoint.name)
		Expect(recorder.Body.String()).To(ContainSubstring("List"), endpoint.name)
		Expect(testDao.orderedBy).To(Equal([]string{"name desc"}), endpoint.name)
		Expect(testDao.whereSQL).To(Equal([]string{endpoint.table + ".name LIKE ?"}), endpoint.name)
		Expect(testDao.fetched).To(BeTrue(), endpoint.name)
	}
}
