package dao

import (
	"context"
	"strings"

	"github.com/jinzhu/inflection"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/openshift-online/maestro/pkg/db"
)

type GenericDao interface {
	Fetch(offset int, limit int, resourceList interface{}) error

	GetInstanceDao(ctx context.Context, model interface{}) GenericDao
	Preload(preload string)
	OrderBy(field string, descending bool) bool
	ResolveColumn(field string) (string, bool)
	ColumnNames() []string
	Joins(sql string)
	Group(sql string)
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

// represents a relationship between two tables. They can be joined,
// ON TableName.ColumnName = ForeignTableName.ForeignColumnName
type TableRelation struct {
	TableName         string
	ColumnName        string
	ForeignTableName  string
	ForeignColumnName string
}

func NewGenericDao(sessionFactory *db.SessionFactory) GenericDao {
	return &sqlGenericDao{sessionFactory: sessionFactory}
}

func (d *sqlGenericDao) GetInstanceDao(ctx context.Context, model interface{}) GenericDao {
	return &sqlGenericDao{
		sessionFactory: d.sessionFactory,
		g2:             (*d.sessionFactory).New(ctx).Model(model),
	}
}

func (d *sqlGenericDao) Fetch(offset int, limit int, resourceList interface{}) error {
	return d.g2.Debug().Unscoped().Offset(offset).Limit(limit).Find(resourceList).Error
}

func (d *sqlGenericDao) Preload(preload string) {
	d.g2 = d.g2.Preload(preload)
}

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

func (d *sqlGenericDao) Joins(sql string) {
	d.g2 = d.g2.Joins(sql)
}

func (d *sqlGenericDao) Group(sql string) {
	d.g2 = d.g2.Group(sql)
}

func (d *sqlGenericDao) Where(sql string, values []interface{}) {
	d.g2 = d.g2.Where(sql, values...)
}

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

// Gorm finishers (Take, First, Last, etc.) are not idempotent
// Use a new session to execute these checks
func (d *sqlGenericDao) Validate(resourceList interface{}) error {
	if err := d.g2.Session(&gorm.Session{DryRun: false}).Take(resourceList).Error; err != nil {
		return err
	}
	return nil
}

func (d *sqlGenericDao) GetTableName() string {
	return db.GetTableName(d.g2)
}

// extract the relation from the api model
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
