package dao

import (
	"context"
	"strings"

	"github.com/jinzhu/inflection"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/openshift-online/maestro/pkg/db"
)

// GenericDao exposes schema-aware query operations for list endpoints.
type GenericDao interface {
	Fetch(offset int, limit int, resourceList interface{}) error

	GetInstanceDao(ctx context.Context, model interface{}) GenericDao
	Preload(preload string)
	OrderBy(field string, descending bool) bool
	ResolveColumn(field string) (string, bool)
	ColumnNames() []string
	Joins(relation TableRelation)
	Group(columns ...clause.Column)
	Where(sql string, values []interface{})
	Count(model interface{}, total *int64)
	Validate(resourceList interface{}) error

	GetTableName() string
	GetTableRelation(fieldName string) (TableRelation, bool)
}

var _ GenericDao = &sqlGenericDao{}

type sqlGenericDao struct {
	sessionFactory *db.SessionFactory
	g2             *gorm.DB
}

// TableRelation represents a relationship between two tables. They can be joined,
// ON TableName.ColumnName = ForeignTableName.ForeignColumnName
type TableRelation struct {
	TableName         string
	ColumnName        string
	ForeignTableName  string
	ForeignColumnName string
}

// NewGenericDao creates a generic DAO backed by the provided session factory.
func NewGenericDao(sessionFactory *db.SessionFactory) GenericDao {
	return &sqlGenericDao{sessionFactory: sessionFactory}
}

// GetInstanceDao returns a DAO session scoped to the supplied model.
func (d *sqlGenericDao) GetInstanceDao(ctx context.Context, model interface{}) GenericDao {
	return &sqlGenericDao{
		sessionFactory: d.sessionFactory,
		g2:             (*d.sessionFactory).New(ctx).Model(model),
	}
}

// Fetch loads a page of records into resourceList.
func (d *sqlGenericDao) Fetch(offset int, limit int, resourceList interface{}) error {
	return d.g2.Debug().Unscoped().Offset(offset).Limit(limit).Find(resourceList).Error
}

// Preload adds a model association to preload when fetching records.
func (d *sqlGenericDao) Preload(preload string) {
	d.g2 = d.g2.Preload(preload)
}

// OrderBy adds an ordering for a field resolved from the model schema.
func (d *sqlGenericDao) OrderBy(field string, descending bool) bool {
	column, ok := d.resolveColumn(field, false)
	if !ok {
		return false
	}
	d.g2 = d.g2.Order(clause.OrderByColumn{Column: column, Desc: descending})
	return true
}

// ResolveColumn resolves a field name to its canonical database column,
// e.g. "CreatedAt" -> "created_at" or "resources.CreatedAt" ->
// "resources.created_at". The second return value is false when the field
// does not resolve to a column of the model (or of a related model), so
// callers can reject user-controlled identifiers before they reach SQL.
func (d *sqlGenericDao) ResolveColumn(field string) (string, bool) {
	column, ok := d.resolveColumn(field, true)
	if !ok {
		return "", false
	}
	if column.Table != "" {
		return column.Table + "." + column.Name, true
	}
	return column.Name, true
}

// ColumnNames returns the database column names declared by the model schema.
func (d *sqlGenericDao) ColumnNames() []string {
	if d.g2 == nil || d.g2.Statement == nil || d.g2.Statement.Parse(d.g2.Statement.Model) != nil || d.g2.Statement.Schema == nil {
		return nil
	}
	return append([]string(nil), d.g2.Statement.Schema.DBNames...)
}

func (d *sqlGenericDao) resolveColumn(name string, allowRelated bool) (clause.Column, bool) {
	if d.g2 == nil || d.g2.Statement == nil || d.g2.Statement.Parse(d.g2.Statement.Model) != nil || d.g2.Statement.Schema == nil {
		return clause.Column{}, false
	}

	parts := strings.Split(name, ".")
	if len(parts) > 2 || len(parts) == 0 {
		return clause.Column{}, false
	}

	rootSchema := d.g2.Statement.Schema
	fieldName := parts[0]
	fieldSchema := rootSchema
	if len(parts) == 2 {
		tableName := parts[0]
		fieldName = parts[1]
		if tableName != rootSchema.Table {
			if !allowRelated {
				return clause.Column{}, false
			}
			fieldSchema = nil
			for _, relation := range rootSchema.Relationships.Relations {
				if relation.FieldSchema != nil && relation.FieldSchema.Table == tableName {
					fieldSchema = relation.FieldSchema
					break
				}
			}
			if fieldSchema == nil {
				return clause.Column{}, false
			}
		}
	}

	field := fieldSchema.LookUpField(fieldName)
	if field == nil || field.DBName == "" {
		return clause.Column{}, false
	}

	column := clause.Column{Name: field.DBName}
	if len(parts) == 2 {
		column.Table = parts[0]
	}
	return column, true
}

// Joins adds a left join using schema-derived table and column identifiers.
// clause.Join is only a clause.Expression, so passing it to Clauses directly
// would render the join as a WHERE condition; joins must extend the FROM
// clause, and repeated calls accumulate instead of replacing each other.
func (d *sqlGenericDao) Joins(relation TableRelation) {
	join := clause.Join{
		Type:  clause.LeftJoin,
		Table: clause.Table{Name: relation.ForeignTableName},
		ON: clause.Where{Exprs: []clause.Expression{
			clause.Eq{
				Column: clause.Column{Table: relation.ForeignTableName, Name: relation.ForeignColumnName},
				Value:  clause.Column{Table: relation.TableName, Name: relation.ColumnName},
			},
			clause.Eq{
				Column: clause.Column{Table: relation.ForeignTableName, Name: "deleted_at"},
				Value:  nil,
			},
		}},
	}
	from := clause.From{Joins: []clause.Join{join}}
	if existing, ok := d.g2.Statement.Clauses["FROM"].Expression.(clause.From); ok {
		from.Joins = append(existing.Joins, join)
	}
	d.g2 = d.g2.Clauses(from)
}

// Group adds a GROUP BY clause using structured column identifiers.
func (d *sqlGenericDao) Group(columns ...clause.Column) {
	d.g2 = d.g2.Clauses(clause.GroupBy{Columns: columns})
}

// Where adds a predicate and its bound values to the query.
func (d *sqlGenericDao) Where(sql string, values []interface{}) {
	d.g2 = d.g2.Where(sql, values...)
}

// Count records matching the current query into total.
func (d *sqlGenericDao) Count(model interface{}, total *int64) {
	g2 := d.g2.Session(&gorm.Session{DryRun: false}).Model(model)
	// There is no need in ORDER BY, GROUP BY and LIMIT in order to count records
	order, oko := g2.Statement.Clauses["ORDER BY"]
	if oko {
		delete(g2.Statement.Clauses, "ORDER BY")
	}
	group, okg := g2.Statement.Clauses["GROUP BY"]
	if okg {
		delete(g2.Statement.Clauses, "GROUP BY")
	}
	limit, okl := g2.Statement.Clauses["LIMIT"]
	if okl {
		delete(g2.Statement.Clauses, "LIMIT")
	}
	g2.Unscoped().Count(total)
	if oko {
		g2.Statement.Clauses["ORDER BY"] = order
	}
	if okg {
		g2.Statement.Clauses["GROUP BY"] = group
	}
	if okl {
		g2.Statement.Clauses["LIMIT"] = limit
	}
}

// Validate checks whether at least one matching record exists.
// GORM finishers (Take, First, Last, etc.) are not idempotent, so this uses a
// new session to execute the check.
func (d *sqlGenericDao) Validate(resourceList interface{}) error {
	if err := d.g2.Session(&gorm.Session{DryRun: false}).Take(resourceList).Error; err != nil {
		return err
	}
	return nil
}

// GetTableName returns the table name from the model schema.
func (d *sqlGenericDao) GetTableName() string {
	return db.GetTableName(d.g2)
}

// GetTableRelation returns the model association corresponding to fieldName.
func (d *sqlGenericDao) GetTableRelation(fieldName string) (TableRelation, bool) {
	// try singular
	fieldName = strings.ToUpper(fieldName[:1]) + fieldName[1:]
	table := inflection.Singular(fieldName)
	association := d.g2.Association(table)
	// the relation must exist in the model
	if association.Relationship == nil {
		// try plural
		table = inflection.Plural(fieldName)
		association = d.g2.Association(table)
		if association.Relationship == nil {
			return TableRelation{}, false
		}
	}

	if association.Relationship.Type != "belongs_to" && association.Relationship.Type != "has_many" {
		// we don't use has_one or many_to_many relations
		return TableRelation{}, false
	}

	columnName := association.Relationship.References[0].ForeignKey.DBName
	foreignColumnName := association.Relationship.References[0].PrimaryKey.DBName
	if association.Relationship.Type == "has_many" {
		columnName = association.Relationship.References[0].PrimaryKey.DBName
		foreignColumnName = association.Relationship.References[0].ForeignKey.DBName
	}

	return TableRelation{
		TableName:         association.Relationship.Field.Schema.Table,
		ForeignTableName:  association.Relationship.FieldSchema.Table,
		ForeignColumnName: foreignColumnName,
		ColumnName:        columnName,
	}, true
}
