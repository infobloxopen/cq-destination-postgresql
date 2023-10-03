package services

import (
	"context"

	"github.com/cloudquery/plugin-sdk/v4/schema"
	"github.com/cloudquery/plugin-sdk/v4/transformers"
	"github.com/infobloxopen/cq-plugin-dest-postgres/cq-source-test/data"
)

func SampleTable() *schema.Table {
	return &schema.Table{
		Name:      "test_data",
		Resolver:  fetchData,
		Transform: transformers.TransformWithStruct(data.Data{}, transformers.WithPrimaryKeys("Id")),
	}
}

func fetchData(ctx context.Context, meta schema.ClientMeta, parent *schema.Resource, res chan<- any) error {
	testData := []data.Data{
		{Id: 1, Name: "John", Age: 20},
		{Id: 2, Name: "Jane", Age: 21},
	}

	for _, d := range testData {
		res <- d
	}

	return nil
}
