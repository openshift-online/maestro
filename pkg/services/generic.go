package services

import (
	"context"
	e "errors"
	"fmt"
	"reflect"
	"strings"

	"gorm.io/gorm"

	"github.com/Masterminds/squirrel"
	"github.com/yaacov/tree-search-language/pkg/tsl"
	"github.com/yaacov/tree-search-language/pkg/walkers/ident"
	sqlFilter "github.com/yaacov/tree-search-language/pkg/walkers/sql"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/db"
	"github.com/openshift-online/maestro/pkg/errors"
	"github.com/openshift-online/maestro/pkg/logger"
)

type GenericService interface {
	List(ctx context.Context, username string, args *ListArguments, resourceList interface{}) (*api.PagingMeta, *errors.ServiceError)
}

func NewGenericService(genericDao dao.GenericDao) GenericService {
	return &sqlGenericService{genericDao: genericDao}
}

var _ GenericService = &sqlGenericService{}

type sqlGenericService struct {
	genericDao dao.GenericDao
}

var (
	SearchDisallowedFields = map[string]map[string]string{}
	allFieldsAllowed       = map[string]string{}
)

// wrap all needed pieces for the LIST funciton
type listContext struct {
	ctx              context.Context
	args             *ListArguments
	username         string
	pagingMeta       *api.PagingMeta
	ulog             *logger.OCMLogger
	resourceList     interface{}
	disallowedFields *map[string]string
	resourceType     string
	joins            map[string]dao.TableRelation
	groupBy          []string
	set              map[string]struct{}
}

func (s *sqlGenericService) newListContext(ctx context.Context, username string, args *ListArguments, resourceList interface{}) (*listContext, interface{}, *errors.ServiceError) {
	log := logger.NewOCMLogger(ctx)
	resourceModel := reflect.TypeOf(resourceList).Elem().Elem()
	resourceTypeStr := resourceModel.Name()
	if resourceTypeStr == "" {
		return nil, nil, errors.GeneralError("Could not determine resource type")
	}
	disallowedFields := SearchDisallowedFields[resourceTypeStr]
	if disallowedFields == nil {
		disallowedFields = allFieldsAllowed
	}
	args.Search = strings.Trim(args.Search, " ")
	return &listContext{
		ctx:              ctx,
		args:             args,
		username:         username,
		pagingMeta:       &api.PagingMeta{Page: args.Page},
		ulog:             &log,
		resourceList:     resourceList,
		disallowedFields: &disallowedFields,
		resourceType:     resourceTypeStr,
	}, reflect.New(resourceModel).Interface(), nil
}

// resourceList must be a pointer to a slice of database resource objects
func (s *sqlGenericService) List(ctx context.Context, username string, args *ListArguments, resourceList interface{}) (*api.PagingMeta, *errors.ServiceError) {
	listCtx, model, err := s.newListContext(ctx, username, args, resourceList)
	if err != nil {
		return nil, err
	}

	// the ordering for the sub functions matters.
	builders := []listBuilder{
		// build SQL to load related resource. for now, it delegates to gorm.preload.
		s.buildPreload,

		// add "ORDER BY"
		s.buildOrderBy,

		// translate "search" into "WHERE"(s), and "JOIN"(s) if related resource is searched.
		s.buildSearch,

		// TODO: add any custom builder functions
	}

	d := s.genericDao.GetInstanceDao(ctx, model)

	// run all the "builders". they cumulatively add constructs to gorm by the context.
	// it stops when a builder function raises error or signals finished.
	var finished bool
	for _, builderFn := range builders {
		if finished, err = builderFn(listCtx, &d); err != nil {
			return nil, err
		}
		if finished {
			if err = s.loadList(listCtx, &d); err != nil {
				return nil, err
			}
			break
		}
	}
	return listCtx.pagingMeta, nil
}

/*** Define all sub functions in the type of listBuilder ***/
type listBuilder func(*listContext, *dao.GenericDao) (finished bool, err *errors.ServiceError)

func (s *sqlGenericService) buildPreload(listCtx *listContext, d *dao.GenericDao) (bool, *errors.ServiceError) {
	listCtx.set = make(map[string]struct{})

	for _, preload := range listCtx.args.Preloads {
		listCtx.set[preload] = struct{}{}
	}
	// preload each table only once; struct{} doesn't occupy any additional space
	for _, preload := range listCtx.args.Preloads {
		(*d).Preload(preload)
	}
	return false, nil
}

func (s *sqlGenericService) buildOrderBy(listCtx *listContext, d *dao.GenericDao) (bool, *errors.ServiceError) {
	if len(listCtx.args.OrderBy) != 0 {
		orderByArgs, serviceErr := db.ArgsToOrderBy(listCtx.args.OrderBy, *listCtx.disallowedFields)
		if serviceErr != nil {
			return false, serviceErr
		}
		for _, orderByArg := range orderByArgs {
			(*d).OrderBy(orderByArg)
		}
	}
	return false, nil
}

func (s *sqlGenericService) buildSearch(listCtx *listContext, d *dao.GenericDao) (bool, *errors.ServiceError) {
	if listCtx.args.Search == "" {
		s.addJoins(listCtx, d)
		return true, nil
	}

	// create the TSL tree
	tslTree, err := tsl.ParseTSL(listCtx.args.Search)
	if err != nil {
		return false, errors.BadRequest("Failed to parse search query: %s", listCtx.args.Search)
	}
	// find all related tables
	tslTree, serviceErr := s.treeWalkForRelatedTables(listCtx, tslTree, d)
	if serviceErr != nil {
		return false, serviceErr
	}
	// prepend table names to prevent "ambiguous" errors
	tslTree, serviceErr = s.treeWalkForAddingTableName(tslTree, d)
	if serviceErr != nil {
		return false, serviceErr
	}
	// convert to sqlizer
	_, sqlizer, serviceErr := s.treeWalkForSqlizer(listCtx, tslTree)
	if serviceErr != nil {
		return false, serviceErr
	}

	s.addJoins(listCtx, d)

	// parse the search string to SQL WHERE
	sql, values, err := sqlizer.ToSql()
	if err != nil {
		return false, errors.GeneralError(err.Error())
	}
	(*d).Where(sql, values)
	return true, nil
}

// JOIN the tables that appear in the search string
func (s *sqlGenericService) addJoins(listCtx *listContext, d *dao.GenericDao) {
	for _, r := range listCtx.joins {
		if _, ok := listCtx.set[r.ForeignTableName]; ok {
			// skip already included preloads
			continue
		}
		sql := fmt.Sprintf(
			"LEFT JOIN %s ON %s.%s = %s.%s AND %s.deleted_at IS NULL",
			r.ForeignTableName, r.ForeignTableName, r.ForeignColumnName, r.TableName, r.ColumnName, r.ForeignTableName)
		(*d).Joins(sql)

		listCtx.groupBy = append(listCtx.groupBy, r.ForeignTableName+".id")
	}
	if len(listCtx.joins) > 0 {
		// Add base relation
		listCtx.groupBy = append(listCtx.groupBy, (*d).GetTableName()+".id")
		(*d).Group(strings.Join(listCtx.groupBy, ","))
	}

	// Reset list of joins and group by's
	listCtx.joins = map[string]dao.TableRelation{}
}

func (s *sqlGenericService) loadList(listCtx *listContext, d *dao.GenericDao) *errors.ServiceError {
	args := listCtx.args
	ulog := *listCtx.ulog

	(*d).Count(listCtx.resourceList, &listCtx.pagingMeta.Total)

	// Set resourceList to be an empty slice with zero capacity. Real space will be allocated by g2.Find()
	if err := zeroSlice(listCtx.resourceList, 0); err != nil {
		return err
	}

	switch {
	case args.Size > MAX_LIST_SIZE:
		ulog.Warning("A query with a size greater than the maximum was requested.")
	case args.Size < 0:
		ulog.Warning("A query with an unbound size was requested.")
	case args.Size == 0:
		// This early return is not only performant, but also necessary.
		// gorm does not support Limit(0) any longer.
		ulog.Infof("A query with 0 size requested, returning early without collecting any resources from database")
		return nil
	}

	// NOTE: Limit no longer supports '0' size and will cause issues. There is an early return, do not remove it.
	//       https://github.com/go-gorm/gorm/blob/master/clause/limit.go#L18-L21
	if err := (*d).Fetch((args.Page-1)*int(args.Size), int(args.Size), listCtx.resourceList); err != nil {
		if e.Is(err, gorm.ErrRecordNotFound) {
			listCtx.pagingMeta.Size = 0
		} else {
			return errors.GeneralError("Unable to list resources: %s", err)
		}
	}
	listCtx.pagingMeta.Size = int64(reflect.ValueOf(listCtx.resourceList).Elem().Len())

	return nil
}

// Allocate a slice with size 'cap' of the type i
func zeroSlice(i interface{}, cap int64) *errors.ServiceError {
	v := reflect.ValueOf(i)
	if v.Kind() != reflect.Ptr {
		return errors.GeneralError("A non-pointer to a list of resources: %v", v.Type())
	}
	// get the value that the pointer v points to.
	v = v.Elem()
	if v.Kind() != reflect.Slice {
		return errors.GeneralError("A non-slice list of resources")
	}
	v.Set(reflect.MakeSlice(v.Type(), 0, int(cap)))
	return nil
}

// walk the TSL tree looking for fields like, e.g., creator.username, and then:
// (1) look up the related table by its 1st part - creator
// (2) replace it by table name - creator.username -> accounts.username
func (s *sqlGenericService) treeWalkForRelatedTables(listCtx *listContext, tslTree tsl.Node, dao *dao.GenericDao) (tsl.Node, *errors.ServiceError) {
	resourceTable := (*dao).GetTableName()

	walkFn := func(field string) (string, error) {
		fieldParts := strings.Split(field, ".")
		if len(fieldParts) > 1 && fieldParts[0] != resourceTable {
			fieldName := fieldParts[0]
			_, exists := listCtx.joins[fieldName]
			if !exists {
				if relation, ok := (*dao).GetTableRelation(fieldName); ok {
					listCtx.joins[fieldName] = relation
				} else {
					return field, fmt.Errorf("%s is not a related resource of %s", fieldName, listCtx.resourceType)
				}
			}
			//replace by table name
			fieldParts[0] = listCtx.joins[fieldName].ForeignTableName
			return strings.Join(fieldParts, "."), nil
		}
		return field, nil
	}

	tslTree, err := ident.Walk(tslTree, walkFn)
	if err != nil {
		return tslTree, errors.BadRequest(err.Error())
	}

	return tslTree, nil
}

// prepend table name to these "free" identifiers since they could cause "ambiguous" errors
func (s *sqlGenericService) treeWalkForAddingTableName(tslTree tsl.Node, dao *dao.GenericDao) (tsl.Node, *errors.ServiceError) {
	resourceTable := (*dao).GetTableName()

	walkFn := func(field string) (string, error) {
		fieldParts := strings.Split(field, ".")
		if len(fieldParts) == 1 {
			if strings.Contains(field, "->") {
				return field, nil
			}
			return fmt.Sprintf("%s.%s", resourceTable, field), nil
		}
		return field, nil
	}

	tslTree, err := ident.Walk(tslTree, walkFn)
	if err != nil {
		return tslTree, errors.BadRequest(err.Error())
	}

	return tslTree, nil
}

func (s *sqlGenericService) treeWalkForSqlizer(listCtx *listContext, tslTree tsl.Node) (tsl.Node, squirrel.Sqlizer, *errors.ServiceError) {
	// Check field names in tree
	tslTree, serviceErr := db.FieldNameWalk(tslTree, *listCtx.disallowedFields)
	if serviceErr != nil {
		return tslTree, nil, serviceErr
	}

	// Convert the search tree into SQL [Squirrel] filter
	sqlizer, err := sqlFilter.Walk(tslTree)
	if err != nil {
		return tslTree, nil, errors.BadRequest(err.Error())
	}

	return tslTree, sqlizer, nil
}
