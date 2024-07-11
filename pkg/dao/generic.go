package dao

import (
	"context"
	"strings"

	"github.com/jinzhu/inflection"
	"gorm.io/gorm"

	"github.com/openshift-online/maestro/pkg/db"
)

type GenericDao interface {
	Fetch(offset int, limit int, resourceList interface{}) error

	GetInstanceDao(ctx context.Context, model interface{}) GenericDao
	Preload(preload string)
	OrderBy(orderBy string)
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

func (d *sqlGenericDao) OrderBy(orderBy string) {
	d.g2 = d.g2.Order(orderBy)
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
