package client

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/cloudquery/plugin-sdk/v4/message"
	"github.com/cloudquery/plugin-sdk/v4/schema"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// InsertBatch inserts records into the destination table. It forms part of the writer.MixedBatchWriter interface.
func (c *Client) InsertBatch(ctx context.Context, messages message.WriteInserts) error {
	tables, err := tablesFromMessages(messages)
	if err != nil {
		return err
	}

	// This happens when the CLI was invoked with `sync --no-migrate`
	if c.pgTablesToPKConstraints == nil {
		// listTables populates c.pgTablesToPKConstraints
		_, err := c.listTables(ctx)
		if err != nil {
			return err
		}
	}

	tables = c.normalizeTables(tables)
	if err != nil {
		return err
	}

	var sql string
	batch := &pgx.Batch{}
	for _, msg := range messages {
		r := msg.Record
		md := r.Schema().Metadata()
		tableName, ok := md.GetValue(schema.MetadataTableName)
		if !ok {
			return fmt.Errorf("table name not found in metadata")
		}
		table := tables.Get(tableName)
		if table == nil {
			return fmt.Errorf("table %s not found", tableName)
		}
		if len(table.PrimaryKeysIndexes()) > 0 {
			sql = c.upsert(table)
		} else {
			sql = c.insert(table)
		}
		rows := transformValues(r)

		indices := getCustomHashableIndices(table.Columns)
		for _, rowVals := range rows {
			for _, idx := range indices {
				if eUUID, ok := rowVals[idx].(pgtype.UUID); ok {
					eUUID.Bytes = regenerateHash(eUUID.Bytes, c.customCQIDSalt)
					rowVals[idx] = eUUID
				}
			}

			batch.Queue(sql, rowVals...)
		}
		batchSize := batch.Len()
		if batchSize >= c.batchSize {
			br := c.conn.SendBatch(ctx, batch)
			if err := br.Close(); err != nil {
				var pgErr *pgconn.PgError
				if !errors.As(err, &pgErr) {
					// not recoverable error
					return fmt.Errorf("failed to execute batch: %w", err)
				}
				return fmt.Errorf("failed to execute batch with pgerror: %s: %w", pgErrToStr(pgErr), err)
			}
			batch = &pgx.Batch{}
		}
	}

	batchSize := batch.Len()
	if batchSize > 0 {
		br := c.conn.SendBatch(ctx, batch)
		if err := br.Close(); err != nil {
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) {
				// not recoverable error
				return fmt.Errorf("failed to execute batch: %w", err)
			}
			return fmt.Errorf("failed to execute batch with pgerror: %s: %w", pgErrToStr(pgErr), err)
		}
	}
	return nil
}

func (*Client) insert(table *schema.Table) string {
	var sb strings.Builder
	tableName := table.Name
	sb.WriteString("insert into ")
	sb.WriteString(pgx.Identifier{tableName}.Sanitize())
	sb.WriteString(" (")
	columns := table.Columns
	columnsLen := len(columns)
	for i, c := range columns {
		sb.WriteString(pgx.Identifier{c.Name}.Sanitize())
		if i < columnsLen-1 {
			sb.WriteString(",")
		} else {
			sb.WriteString(") values (")
		}
	}
	for i := range columns {
		sb.WriteString(fmt.Sprintf("$%d", i+1))
		if i < columnsLen-1 {
			sb.WriteString(",")
		} else {
			sb.WriteString(")")
		}
	}
	return sb.String()
}

func (c *Client) upsert(table *schema.Table) string {
	var sb strings.Builder

	sb.WriteString(c.insert(table))
	columns := table.Columns
	columnsLen := len(columns)

	constraintName := table.PkConstraintName
	sb.WriteString(" on conflict on constraint ")
	sb.WriteString(pgx.Identifier{constraintName}.Sanitize())
	sb.WriteString(" do update set ")
	for i, column := range columns {
		sb.WriteString(pgx.Identifier{column.Name}.Sanitize())
		sb.WriteString("=excluded.") // excluded references the new values
		sb.WriteString(pgx.Identifier{column.Name}.Sanitize())
		if i < columnsLen-1 {
			sb.WriteString(",")
		} else {
			sb.WriteString("")
		}
	}

	return sb.String()
}

func pgErrToStr(err *pgconn.PgError) string {
	var sb strings.Builder
	sb.WriteString("severity: ")
	sb.WriteString(err.Severity)
	sb.WriteString(", code: ")
	sb.WriteString(err.Code)
	sb.WriteString(", message: ")
	sb.WriteString(err.Message)
	sb.WriteString(", detail :")
	sb.WriteString(err.Detail)
	sb.WriteString(", hint: ")
	sb.WriteString(err.Hint)
	sb.WriteString(", position: ")
	sb.WriteString(strconv.FormatInt(int64(err.Position), 10))
	sb.WriteString(", internal_position: ")
	sb.WriteString(strconv.FormatInt(int64(err.InternalPosition), 10))
	sb.WriteString(", internal_query: ")
	sb.WriteString(err.InternalQuery)
	sb.WriteString(", where: ")
	sb.WriteString(err.Where)
	sb.WriteString(", schema_name: ")
	sb.WriteString(err.SchemaName)
	sb.WriteString(", table_name: ")
	sb.WriteString(err.TableName)
	sb.WriteString(", column_name: ")
	sb.WriteString(err.ColumnName)
	sb.WriteString(", data_type_name: ")
	sb.WriteString(err.DataTypeName)
	sb.WriteString(", constraint_name: ")
	sb.WriteString(err.ConstraintName)
	sb.WriteString(", file: ")
	sb.WriteString(err.File)
	sb.WriteString(", line: ")
	sb.WriteString(strconv.FormatUint(uint64(err.Line), 10))
	sb.WriteString(", routine: ")
	sb.WriteString(err.Routine)
	return sb.String()
}

func getCustomHashableIndices(columns schema.ColumnList) []int {
	var indices []int
	for i := range columns {
		if columns[i].Name == "_cq_id" || columns[i].Name == "_cq_parent_id" {
			indices = append(indices, i)
		}
	}
	return indices
}

func regenerateHash(val [16]byte, prefix string) [16]byte {
	h := sha256.New()
	h.Write(val[:])
	h.Write([]byte(prefix))
	return [16]byte(uuid.NewSHA1(uuid.UUID{}, h.Sum(nil)))
}
