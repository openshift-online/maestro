package services

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"github.com/yaacov/tree-search-language/pkg/tsl"
	"gorm.io/gorm/clause"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/errors"
)

func TestSQLTranslation(t *testing.T) {
	RegisterTestingT(t)
	g := newGenericTestDao("resources",
		"id", "created_at", "updated_at", "deleted_at",
		"version", "source", "consumer_name", "type",
		"payload", "status", "name",
	)
	genericService := sqlGenericService{genericDao: g}

	// ill-formatted search or disallowed fields should be rejected
	tests := []map[string]interface{}{
		{
			"search": "garbage",
			"error":  "maestro-21: failed to parse the search query",
		},
		{
			"search": "id in ('123')",
			"error":  "maestro-21: a search field is not valid",
		},
	}
	for _, test := range tests {
		list := []api.Resource{}
		search := test["search"].(string)
		errorMsg := test["error"].(string)
		listCtx, model, serviceErr := genericService.newListContext(context.Background(), "", &ListArguments{Search: search}, &list)
		Expect(serviceErr).ToNot(HaveOccurred())
		d := g.GetInstanceDao(context.Background(), model)
		disallowedFields := map[string]string{"id": "id"}
		listCtx.disallowedFields = &disallowedFields
		_, serviceErr = genericService.buildSearch(listCtx, &d)
		Expect(serviceErr).To(HaveOccurred())
		Expect(serviceErr.Code).To(Equal(errors.ErrorBadRequest))
		Expect(serviceErr.Error()).To(Equal(errorMsg))
	}

	// tests for sql parsing
	tests = []map[string]interface{}{
		{
			"search": "name in ('ooo.openshift')",
			"sql":    "name IN (?)",
			"values": ConsistOf("ooo.openshift"),
		},
	}
	for _, test := range tests {
		list := []api.Resource{}
		search := test["search"].(string)
		sqlReal := test["sql"].(string)
		valuesReal := test["values"].(types.GomegaMatcher)
		listCtx, model, serviceErr := genericService.newListContext(context.Background(), "", &ListArguments{Search: search}, &list)
		Expect(serviceErr).ToNot(HaveOccurred())
		d := g.GetInstanceDao(context.Background(), model)
		tslTree, err := tsl.ParseTSL(search)
		Expect(err).ToNot(HaveOccurred())
		_, sqlizer, serviceErr := genericService.treeWalkForSqlizer(listCtx, tslTree, &d)
		Expect(serviceErr).ToNot(HaveOccurred())
		sql, values, err := sqlizer.ToSql()
		Expect(err).ToNot(HaveOccurred())
		Expect(sql).To(Equal(sqlReal))
		Expect(values).To(valuesReal)
	}

	// User-controlled identifiers must resolve to model columns before reaching SQL.
	searchTests := []struct {
		search string
		valid  bool
	}{
		{search: "name = 'consumer'", valid: true},
		{search: "properties.foo = 'bar'", valid: false},
		{search: `resources."name' OR 1=1 --" = 'x'`, valid: false},
		{search: "not_a_resource_column = 'x'", valid: false},
	}
	for _, test := range searchTests {
		list := []api.Resource{}
		listCtx, model, serviceErr := genericService.newListContext(context.Background(), "", &ListArguments{Search: test.search}, &list)
		Expect(serviceErr).ToNot(HaveOccurred())
		d := g.GetInstanceDao(context.Background(), model)
		_, serviceErr = genericService.buildSearch(listCtx, &d)
		if test.valid {
			Expect(serviceErr).ToNot(HaveOccurred(), test.search)
		} else {
			Expect(serviceErr).To(HaveOccurred(), test.search)
			Expect(serviceErr.Code).To(Equal(errors.ErrorBadRequest))
		}
	}

	orderTests := []struct {
		orderBy string
		valid   bool
	}{
		{orderBy: "name desc", valid: true},
		{orderBy: "created_at asc", valid: true},
		{orderBy: "missing_column", valid: false},
		{orderBy: "name desc; DROP TABLE resources", valid: false},
		{orderBy: `name"; DROP TABLE resources --`, valid: false},
	}
	for _, test := range orderTests {
		list := []api.Resource{}
		listCtx, model, serviceErr := genericService.newListContext(context.Background(), "", &ListArguments{OrderBy: []string{test.orderBy}}, &list)
		Expect(serviceErr).ToNot(HaveOccurred())
		d := g.GetInstanceDao(context.Background(), model)
		_, serviceErr = genericService.buildOrderBy(listCtx, &d)
		if test.valid {
			Expect(serviceErr).ToNot(HaveOccurred(), test.orderBy)
		} else {
			Expect(serviceErr).To(HaveOccurred(), test.orderBy)
			Expect(serviceErr.Code).To(Equal(errors.ErrorBadRequest))
		}
	}

	jsonbTests := []struct {
		search string
		valid  bool
	}{
		{search: `resources.payload -> 'metadata' -> 'labels' @> '{"cluster":"east"}'`, valid: true},
		{search: `unknown_column -> 'cluster' ->> 'region' = 'east'`, valid: false},
	}
	for _, test := range jsonbTests {
		list := []api.Resource{}
		listCtx, model, serviceErr := genericService.newListContext(context.Background(), "", &ListArguments{Search: test.search}, &list)
		Expect(serviceErr).ToNot(HaveOccurred())
		d := g.GetInstanceDao(context.Background(), model)
		_, serviceErr = genericService.buildSearch(listCtx, &d)
		if test.valid {
			Expect(serviceErr).ToNot(HaveOccurred(), test.search)
		} else {
			Expect(serviceErr).To(HaveOccurred(), test.search)
			Expect(serviceErr.Code).To(Equal(errors.ErrorBadRequest))
		}
	}
}

var resourceColumns = []string{
	"id", "created_at", "updated_at", "deleted_at",
	"version", "source", "consumer_name", "type",
	"payload", "status", "name",
}

var consumerColumns = []string{"id", "created_at", "updated_at", "deleted_at", "name", "labels"}

// listTargets mirrors the two REST list endpoints that accept search and
// orderBy query parameters: /api/maestro/v1/resource-bundles and
// /api/maestro/v1/consumers.
func listTargets() []struct {
	name    string
	table   string
	columns []string
	newList func() interface{}
} {
	return []struct {
		name    string
		table   string
		columns []string
		newList func() interface{}
	}{
		{
			name:    "resource-bundles",
			table:   "resources",
			columns: resourceColumns,
			newList: func() interface{} { return &[]api.Resource{} },
		},
		{
			name:    "consumers",
			table:   "consumers",
			columns: consumerColumns,
			newList: func() interface{} { return &[]api.Consumer{} },
		},
	}
}

// TestListBlocksOrderBySQLInjection attempts SQL identifier injection through
// the orderBy query parameter of both list endpoints. Every payload must be
// rejected with a bad request before any query fragment reaches the DAO.
func TestListBlocksOrderBySQLInjection(t *testing.T) {
	RegisterTestingT(t)

	payloads := []string{
		"(SELECT/**/1)",
		"lower(name)",
		`name" OR 1=1 --`,
		"name;DROP TABLE resources",
		"name desc; DROP TABLE resources",
		"resources.name.extra desc",
	}

	for _, target := range listTargets() {
		for _, payload := range payloads {
			testDao := newGenericTestDao(target.table, target.columns...)
			service := sqlGenericService{genericDao: testDao}
			args := NewListArguments(url.Values{"orderBy": {payload}})

			_, serviceErr := service.List(context.Background(), "", args, target.newList())

			Expect(serviceErr).To(HaveOccurred(), "%s: orderBy=%q", target.name, payload)
			Expect(serviceErr.Code).To(Equal(errors.ErrorBadRequest), "%s: orderBy=%q", target.name, payload)
			Expect(serviceErr.HttpCode).To(Equal(http.StatusBadRequest), "%s: orderBy=%q", target.name, payload)
			// the exploit must be rejected before any query is built or executed
			Expect(testDao.orderedBy).To(BeEmpty(), "%s: orderBy=%q", target.name, payload)
			Expect(testDao.whereSQL).To(BeEmpty(), "%s: orderBy=%q", target.name, payload)
			Expect(testDao.counted).To(BeFalse(), "%s: orderBy=%q", target.name, payload)
			Expect(testDao.fetched).To(BeFalse(), "%s: orderBy=%q", target.name, payload)
		}

		// a valid ordering request still works once the attacks are blocked
		testDao := newGenericTestDao(target.table, target.columns...)
		service := sqlGenericService{genericDao: testDao}
		args := NewListArguments(url.Values{"orderBy": {"name desc"}})

		_, serviceErr := service.List(context.Background(), "", args, target.newList())

		Expect(serviceErr).ToNot(HaveOccurred(), target.name)
		Expect(testDao.orderedBy).To(Equal([]string{"name desc"}), target.name)
		Expect(testDao.fetched).To(BeTrue(), target.name)
	}
}

// TestListBlocksSearchIdentifierInjection attempts SQL identifier injection
// through the search query parameter of both list endpoints, including the
// quoted TSL identifier forms used by the properties.* exploit. Every payload
// must be rejected with a bad request before any query fragment reaches the DAO.
func TestListBlocksSearchIdentifierInjection(t *testing.T) {
	RegisterTestingT(t)

	payloads := []string{
		`properties.foo = 'bar'`,
		`"name' OR 1=1 --" = 'x'`,
		"`name' OR 1=1 --` = 'x'",
		"not_a_column = 'x'",
		"unknown_column -> 'k' ->> 'k2' = 'v'",
	}

	for _, target := range listTargets() {
		for _, payload := range payloads {
			testDao := newGenericTestDao(target.table, target.columns...)
			service := sqlGenericService{genericDao: testDao}
			args := NewListArguments(url.Values{"search": {payload}})

			_, serviceErr := service.List(context.Background(), "", args, target.newList())

			Expect(serviceErr).To(HaveOccurred(), "%s: search=%q", target.name, payload)
			Expect(serviceErr.Code).To(Equal(errors.ErrorBadRequest), "%s: search=%q", target.name, payload)
			Expect(serviceErr.HttpCode).To(Equal(http.StatusBadRequest), "%s: search=%q", target.name, payload)
			// the exploit must be rejected before any query is built or executed
			Expect(testDao.whereSQL).To(BeEmpty(), "%s: search=%q", target.name, payload)
			Expect(testDao.orderedBy).To(BeEmpty(), "%s: search=%q", target.name, payload)
			Expect(testDao.counted).To(BeFalse(), "%s: search=%q", target.name, payload)
			Expect(testDao.fetched).To(BeFalse(), "%s: search=%q", target.name, payload)
		}

		// a valid search still works once the attacks are blocked
		testDao := newGenericTestDao(target.table, target.columns...)
		service := sqlGenericService{genericDao: testDao}
		args := NewListArguments(url.Values{"search": {"name like 'c%'"}})

		_, serviceErr := service.List(context.Background(), "", args, target.newList())

		Expect(serviceErr).ToNot(HaveOccurred(), target.name)
		Expect(testDao.whereSQL).To(Equal([]string{target.table + ".name LIKE ?"}), target.name)
		Expect(testDao.fetched).To(BeTrue(), target.name)
	}
}

// TestListJSONBSearchParameterizesKeys proves that user-controlled JSONB path
// keys are passed to the database as bound parameters rather than SQL text,
// and that JSONB searches on unknown columns are rejected.
func TestListJSONBSearchParameterizesKeys(t *testing.T) {
	RegisterTestingT(t)

	t.Run("malicious JSONB key is bound as a parameter", func(t *testing.T) {
		RegisterTestingT(t)
		testDao := newGenericTestDao("resources", resourceColumns...)
		service := sqlGenericService{genericDao: testDao}
		search := `payload -> 'x\' OR \'1\'=\'1' ->> 'y' = 'v'`
		args := NewListArguments(url.Values{"search": {search}})

		_, serviceErr := service.List(context.Background(), "", args, &[]api.Resource{})

		Expect(serviceErr).ToNot(HaveOccurred())
		Expect(testDao.whereSQL).To(HaveLen(1))
		Expect(testDao.whereSQL[0]).To(Equal("payload -> ? ->> ? = ?"))
		Expect(testDao.whereValues[0]).To(HaveLen(3))
		Expect(testDao.whereValues[0][0]).To(Equal("x' OR '1'='1"))
		Expect(testDao.whereValues[0][1]).To(Equal("y"))
		Expect(testDao.whereValues[0][2]).To(Equal("v"))
	})

	t.Run("valid JSONB containment search", func(t *testing.T) {
		RegisterTestingT(t)
		testDao := newGenericTestDao("resources", resourceColumns...)
		service := sqlGenericService{genericDao: testDao}
		search := `payload -> 'metadata' -> 'labels' @> '{"cluster":"east"}'`
		args := NewListArguments(url.Values{"search": {search}})

		_, serviceErr := service.List(context.Background(), "", args, &[]api.Resource{})

		Expect(serviceErr).ToNot(HaveOccurred())
		Expect(testDao.whereSQL).To(HaveLen(1))
		Expect(testDao.whereSQL[0]).To(Equal("payload -> ? -> ? @> ?"))
		Expect(testDao.whereValues[0]).To(Equal([]interface{}{"metadata", "labels", `{"cluster":"east"}`}))
	})
}

// TestListCanonicalizesFieldAliases proves that field-name aliases (the Go
// struct field names accepted by GORM's schema lookup) are canonicalized to
// database column names, and that the disallowlist is enforced against the
// canonical name so aliases cannot bypass it.
func TestListCanonicalizesFieldAliases(t *testing.T) {
	RegisterTestingT(t)

	t.Run("search aliases resolve to canonical columns", func(t *testing.T) {
		RegisterTestingT(t)
		testDao := newGenericTestDao("resources", resourceColumns...)
		service := sqlGenericService{genericDao: testDao}
		args := NewListArguments(url.Values{"search": {"CreatedAt = '2026-01-01'"}})

		_, serviceErr := service.List(context.Background(), "", args, &[]api.Resource{})

		Expect(serviceErr).ToNot(HaveOccurred())
		Expect(testDao.whereSQL).To(Equal([]string{"resources.created_at = ?"}))
	})

	t.Run("orderBy aliases resolve to canonical columns", func(t *testing.T) {
		RegisterTestingT(t)
		testDao := newGenericTestDao("resources", resourceColumns...)
		service := sqlGenericService{genericDao: testDao}
		args := NewListArguments(url.Values{"orderBy": {"CreatedAt desc"}})

		_, serviceErr := service.List(context.Background(), "", args, &[]api.Resource{})

		Expect(serviceErr).ToNot(HaveOccurred())
		Expect(testDao.orderedBy).To(Equal([]string{"created_at desc"}))
	})

	t.Run("disallowed fields cannot be bypassed with aliases", func(t *testing.T) {
		RegisterTestingT(t)
		testDao := newGenericTestDao("resources", resourceColumns...)
		service := sqlGenericService{genericDao: testDao}
		disallowedFields := map[string]string{"id": "id"}

		list := []api.Resource{}
		listCtx, model, serviceErr := service.newListContext(context.Background(), "", &ListArguments{Search: "ID = '123'"}, &list)
		Expect(serviceErr).ToNot(HaveOccurred())
		listCtx.disallowedFields = &disallowedFields
		d := testDao.GetInstanceDao(context.Background(), model)
		_, serviceErr = service.buildSearch(listCtx, &d)
		Expect(serviceErr).To(HaveOccurred())
		Expect(serviceErr.Code).To(Equal(errors.ErrorBadRequest))
		Expect(testDao.whereSQL).To(BeEmpty())

		listCtx, model, serviceErr = service.newListContext(context.Background(), "", &ListArguments{OrderBy: []string{"ID"}}, &list)
		Expect(serviceErr).ToNot(HaveOccurred())
		listCtx.disallowedFields = &disallowedFields
		d = testDao.GetInstanceDao(context.Background(), model)
		_, serviceErr = service.buildOrderBy(listCtx, &d)
		Expect(serviceErr).To(HaveOccurred())
		Expect(serviceErr.Code).To(Equal(errors.ErrorBadRequest))
		Expect(testDao.orderedBy).To(BeEmpty())
	})
}

type genericTestDao struct {
	table   string
	columns map[string]struct{}

	// recorded calls, used by the injection tests to prove that no query is
	// built or executed for a rejected request
	whereSQL    []string
	whereValues [][]interface{}
	orderedBy   []string
	counted     bool
	fetched     bool
}

var _ dao.GenericDao = (*genericTestDao)(nil)

func newGenericTestDao(table string, columns ...string) *genericTestDao {
	columnSet := make(map[string]struct{}, len(columns))
	for _, column := range columns {
		columnSet[column] = struct{}{}
	}
	return &genericTestDao{table: table, columns: columnSet}
}

func (d *genericTestDao) Fetch(int, int, interface{}) error {
	d.fetched = true
	return nil
}

func (d *genericTestDao) GetInstanceDao(context.Context, interface{}) dao.GenericDao { return d }

func (*genericTestDao) Preload(string) {}

func (d *genericTestDao) OrderBy(field string, descending bool) bool {
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

// ResolveColumn mirrors the real DAO: the field is looked up through a
// GORM-style naming strategy (so the Go field alias "CreatedAt" canonicalizes
// to the column "created_at") and the canonical column name is returned,
// qualified when the input was qualified.
func (d *genericTestDao) ResolveColumn(field string) (string, bool) {
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

func (d *genericTestDao) ColumnNames() []string {
	columns := make([]string, 0, len(d.columns))
	for column := range d.columns {
		columns = append(columns, column)
	}
	return columns
}

func (d *genericTestDao) Joins(dao.TableRelation) {}

func (d *genericTestDao) Group(...clause.Column) {}

func (d *genericTestDao) Where(sql string, values []interface{}) {
	d.whereSQL = append(d.whereSQL, sql)
	d.whereValues = append(d.whereValues, values)
}

func (d *genericTestDao) Count(model interface{}, total *int64) {
	d.counted = true
	*total = 0
}

func (*genericTestDao) Validate(interface{}) error { return nil }

func (d *genericTestDao) GetTableName() string { return d.table }

func (*genericTestDao) GetTableRelation(string) (dao.TableRelation, bool) {
	return dao.TableRelation{}, false
}
