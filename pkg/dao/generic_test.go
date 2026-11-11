package dao

import (
	"strings"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/openshift-online/maestro/pkg/api"
)

func TestOrderByUsesResolvedStructuredColumns(t *testing.T) {
	g2, err := gorm.Open(postgres.New(postgres.Config{
		DSN: "host=localhost user=postgres dbname=maestro sslmode=disable",
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("failed to create dry-run GORM session: %v", err)
	}

	d := &sqlGenericDao{g2: g2.Model(&api.Resource{})}
	if !d.OrderBy("name", true) {
		t.Fatal("expected model column to resolve")
	}
	if d.OrderBy(`name"; DROP TABLE resources --`, false) {
		t.Fatal("expected malformed column to be rejected")
	}

	query := d.g2.Find(&[]api.Resource{}).Statement.SQL.String()
	if !strings.Contains(query, `ORDER BY "name" DESC`) {
		t.Fatalf("expected safely quoted descending order column, got SQL: %s", query)
	}
}

func TestOrderByCanonicalizesFieldAliases(t *testing.T) {
	g2, err := gorm.Open(postgres.New(postgres.Config{
		DSN: "host=localhost user=postgres dbname=maestro sslmode=disable",
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("failed to create dry-run GORM session: %v", err)
	}

	// "CreatedAt" is the Go struct field name; it must be emitted as the
	// canonical, quoted database column rather than passed through verbatim.
	// GORM renders the ascending direction implicitly.
	d := &sqlGenericDao{g2: g2.Model(&api.Resource{})}
	if !d.OrderBy("CreatedAt", false) {
		t.Fatal("expected field alias to resolve")
	}

	query := d.g2.Find(&[]api.Resource{}).Statement.SQL.String()
	if !strings.Contains(query, `ORDER BY "created_at"`) || strings.Contains(query, "DESC") {
		t.Fatalf("expected canonical quoted ascending order column, got SQL: %s", query)
	}
}

func TestResolveColumnCanonicalizesAndRejects(t *testing.T) {
	g2, err := gorm.Open(postgres.New(postgres.Config{
		DSN: "host=localhost user=postgres dbname=maestro sslmode=disable",
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("failed to create dry-run GORM session: %v", err)
	}

	d := &sqlGenericDao{g2: g2.Model(&api.Resource{})}

	resolved, ok := d.ResolveColumn("resources.payload")
	if !ok || resolved != "resources.payload" {
		t.Fatalf("expected qualified model column to resolve, got %q ok=%v", resolved, ok)
	}

	resolved, ok = d.ResolveColumn("CreatedAt")
	if !ok || resolved != "created_at" {
		t.Fatalf("expected alias to canonicalize to created_at, got %q ok=%v", resolved, ok)
	}

	resolved, ok = d.ResolveColumn("resources.CreatedAt")
	if !ok || resolved != "resources.created_at" {
		t.Fatalf("expected qualified alias to canonicalize, got %q ok=%v", resolved, ok)
	}

	for _, field := range []string{
		`resources."payload"; DROP TABLE resources --`,
		"not_a_column",
		"resources.not_a_column",
		"resources.name.extra",
		"",
	} {
		if resolved, ok := d.ResolveColumn(field); ok {
			t.Fatalf("expected %q to fail schema lookup, got %q", field, resolved)
		}
	}
}

func TestColumnNamesAreDerivedFromModelSchema(t *testing.T) {
	g2, err := gorm.Open(postgres.New(postgres.Config{
		DSN: "host=localhost user=postgres dbname=maestro sslmode=disable",
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("failed to create dry-run GORM session: %v", err)
	}

	d := &sqlGenericDao{g2: g2.Model(&api.Resource{})}
	if _, ok := d.ResolveColumn(`resources."payload"; DROP TABLE resources --`); ok {
		t.Fatal("expected SQL expression to fail schema lookup")
	}

	columns := d.ColumnNames()
	if len(columns) == 0 {
		t.Fatal("expected model schema columns")
	}
	for _, column := range columns {
		if column == "payload" {
			return
		}
	}
	t.Fatalf("expected payload in model columns, got %v", columns)
}

func TestIsJSONColumnUsesModelSchema(t *testing.T) {
	g2, err := gorm.Open(postgres.New(postgres.Config{
		DSN: "host=localhost user=postgres dbname=maestro sslmode=disable",
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("failed to create dry-run GORM session: %v", err)
	}

	resourceDAO := &sqlGenericDao{g2: g2.Model(&api.Resource{})}
	for _, field := range []string{"payload", "status"} {
		if !resourceDAO.IsJSONColumn(field) {
			t.Errorf("expected %s to be recognized as a JSON column", field)
		}
	}
	if !resourceDAO.IsJSONColumn("PAYLOAD") {
		t.Error("expected uppercase payload to be recognized as a JSON column")
	}
	if resourceDAO.IsJSONColumn("name") {
		t.Error("expected resource name to be recognized as a non-JSON column")
	}

	consumerDAO := &sqlGenericDao{g2: g2.Model(&api.Consumer{})}
	if !consumerDAO.IsJSONColumn("labels") {
		t.Error("expected consumer labels to be recognized as a JSON column")
	}
	if consumerDAO.IsJSONColumn("name") {
		t.Error("expected consumer name to be recognized as a non-JSON column")
	}
}

// TestJoinsAndGroupsUseStructuredColumns checks that relation identifiers are
// quoted, that joins extend the FROM clause rather than the WHERE clause, and
// that repeated joins accumulate instead of replacing each other.
func TestJoinsAndGroupsUseStructuredColumns(t *testing.T) {
	g2, err := gorm.Open(postgres.New(postgres.Config{
		DSN: "host=localhost user=postgres dbname=maestro sslmode=disable",
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("failed to create dry-run GORM session: %v", err)
	}

	d := &sqlGenericDao{g2: g2.Model(&api.Resource{})}
	d.Joins(TableRelation{
		TableName:         "resources",
		ColumnName:        "consumer_name",
		ForeignTableName:  "consumers",
		ForeignColumnName: "name",
	})
	d.Joins(TableRelation{
		TableName:         "resources",
		ColumnName:        "id",
		ForeignTableName:  "events",
		ForeignColumnName: "source_id",
	})
	d.Where("resources.name = ?", []interface{}{"findme"})
	d.Group(clause.Column{Table: "resources", Name: "id"}, clause.Column{Table: "consumers", Name: "id"})

	query := d.g2.Find(&[]api.Resource{}).Statement.SQL.String()

	// The fragments must appear in this order: a join routed into the WHERE
	// clause, or a join replacing a previously added one, fails the test.
	fragments := []string{
		`FROM "resources"`,
		`LEFT JOIN "consumers" ON "consumers"."name" = "resources"."consumer_name" AND "consumers"."deleted_at" IS NULL`,
		`LEFT JOIN "events" ON "events"."source_id" = "resources"."id" AND "events"."deleted_at" IS NULL`,
		`WHERE resources.name = $1`,
		`GROUP BY "resources"."id","consumers"."id"`,
	}
	last := -1
	for _, fragment := range fragments {
		idx := strings.Index(query, fragment)
		if idx < 0 {
			t.Fatalf("expected SQL fragment %q, got SQL: %s", fragment, query)
		}
		if idx < last {
			t.Fatalf("expected SQL fragment %q after the preceding fragment, got SQL: %s", fragment, query)
		}
		last = idx
	}
}
