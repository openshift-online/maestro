package services

import (
	"context"
	"testing"

	"github.com/openshift-online/maestro/pkg/dao"
	"github.com/openshift-online/maestro/pkg/db"

	"github.com/onsi/gomega/types"
	"github.com/yaacov/tree-search-language/pkg/tsl"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/config"
	"github.com/openshift-online/maestro/pkg/db/db_session"
	"github.com/openshift-online/maestro/pkg/errors"

	. "github.com/onsi/gomega"
)

func TestSQLTranslation(t *testing.T) {
	RegisterTestingT(t)
	dbConfig := config.NewDatabaseConfig()
	err := dbConfig.ReadFiles()
	Expect(err).ToNot(HaveOccurred())
	var dbFactory db.SessionFactory = db_session.NewProdFactory(dbConfig)
	defer dbFactory.Close()

	g := dao.NewGenericDao(&dbFactory)
	genericService := sqlGenericService{genericDao: g}

	// ill-formatted search or disallowed fields should be rejected
	tests := []map[string]interface{}{
		{
			"search": "garbage",
			"error":  "maestro-21: Failed to parse search query: garbage",
		},
		{
			"search": "id in ('123')",
			"error":  "maestro-21: dinosaurs.id is not a valid field name",
		},
	}
	for _, test := range tests {
		list := []api.Dinosaur{}
		search := test["search"].(string)
		errorMsg := test["error"].(string)
		listCtx, model, serviceErr := genericService.newListContext(context.Background(), "", &ListArguments{Search: search}, &list)
		Expect(serviceErr).ToNot(HaveOccurred())
		d := g.GetInstanceDao(context.Background(), model)
		(*listCtx.disallowedFields)["id"] = "id"
		_, serviceErr = genericService.buildSearch(listCtx, &d)
		Expect(serviceErr).To(HaveOccurred())
		Expect(serviceErr.Code).To(Equal(errors.ErrorBadRequest))
		Expect(serviceErr.Error()).To(Equal(errorMsg))
	}

	// tests for sql parsing
	tests = []map[string]interface{}{
		{
			"search": "username in ('ooo.openshift')",
			"sql":    "username IN (?)",
			"values": ConsistOf("ooo.openshift"),
		},
	}
	for _, test := range tests {
		list := []api.Dinosaur{}
		search := test["search"].(string)
		sqlReal := test["sql"].(string)
		valuesReal := test["values"].(types.GomegaMatcher)
		listCtx, _, serviceErr := genericService.newListContext(context.Background(), "", &ListArguments{Search: search}, &list)
		Expect(serviceErr).ToNot(HaveOccurred())
		tslTree, err := tsl.ParseTSL(search)
		Expect(err).ToNot(HaveOccurred())
		_, sqlizer, serviceErr := genericService.treeWalkForSqlizer(listCtx, tslTree)
		Expect(serviceErr).ToNot(HaveOccurred())
		sql, values, err := sqlizer.ToSql()
		Expect(err).ToNot(HaveOccurred())
		Expect(sql).To(Equal(sqlReal))
		Expect(values).To(valuesReal)
	}
}
