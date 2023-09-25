package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
)

type Data struct {
	Id              int
	Age             int
	Name            string
	_cq_id          string
	_cq_source_name string
}

func TestPlugin(t *testing.T) {
	dbDSN := "postgresql://postgres:postgres@0.0.0.0:5432/postgres?sslmode=disable"

	if dsn := os.Getenv("CQ_DEST_PG_TEST_CONN"); dsn != "" {
		dbDSN = dsn
	}

	conn, err := pgx.Connect(context.Background(), dbDSN)
	assert.Nil(t, err)
	defer conn.Close(context.Background())

	_, err = conn.Exec(context.Background(), "drop table if exists test_data;")
	assert.Nil(t, err)

	args := []string{
		"sync",
		"--log-console",
		"--no-log-file",
		"./test_config/config.yml",
	}

	for _, salt := range []string{"test", "test2"} {
		cqCmd := exec.Command("cloudquery", args...)
		cqCmd.Env = []string{
			fmt.Sprintf("CUSTOM_CQID_SALT=%s", salt),
			fmt.Sprintf("SOURCE_NAME=%s", salt),
			"CQ_TELEMETRY_LEVEL=none",
		}

		_, err = cqCmd.CombinedOutput()
		assert.Nil(t, err)
	}

	rows, err := conn.Query(context.Background(), "select id, age, name, _cq_id, _cq_source_name from test_data where id = $1", 1)
	assert.Nil(t, err)

	var dataSlice []Data
	for rows.Next() {
		data := Data{}
		err = rows.Scan(&data.Id, &data.Age, &data.Name, &data._cq_id, &data._cq_source_name)
		assert.Nil(t, err)
		dataSlice = append(dataSlice, data)
	}

	assert.Equal(t, 2, len(dataSlice))
	assert.Equal(t, dataSlice[0].Id, dataSlice[1].Id)
	assert.Equal(t, dataSlice[0].Age, dataSlice[1].Age)
	assert.Equal(t, dataSlice[0].Name, dataSlice[1].Name)
	assert.NotEqual(t, dataSlice[0]._cq_id, dataSlice[1]._cq_id)
	assert.NotEqual(t, dataSlice[0]._cq_source_name, dataSlice[1]._cq_source_name)
}
