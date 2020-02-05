// Copyright 2019 Liquidata, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sqle

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	sqle "github.com/src-d/go-mysql-server"
	"github.com/src-d/go-mysql-server/sql"
	"vitess.io/vitess/go/vt/sqlparser"

	"github.com/liquidata-inc/dolt/go/libraries/doltcore/doltdb"
	"github.com/liquidata-inc/dolt/go/libraries/doltcore/env"
)

// Executes all the SQL non-select statements given in the string against the root value given and returns the updated
// root, or an error. Statements in the input string are split by `;\n`
func ExecuteSql(dEnv *env.DoltEnv, root *doltdb.RootValue, statements string) (*doltdb.RootValue, error) {
	engine := sqle.NewDefault()
	db := NewBatchedDatabase("dolt", root, dEnv.DoltDB, dEnv.RepoState)
	engine.AddDatabase(db)

	err := RegisterSchemaFragments(sql.NewContext(context.Background()), engine.Catalog, db)
	if err != nil {
		return nil, err
	}

	for _, query := range strings.Split(statements, ";\n") {
		if len(strings.Trim(query, " ")) == 0 {
			continue
		}

		sqlStatement, err := sqlparser.Parse(query)
		if err != nil {
			return nil, err
		}

		var execErr error
		switch sqlStatement.(type) {
		case *sqlparser.Show:
			return nil, errors.New("Show statements aren't handled")
		case *sqlparser.Select, *sqlparser.OtherRead:
			return nil, errors.New("Select statements aren't handled")
		case *sqlparser.Insert:
			var rowIter sql.RowIter
			_, rowIter, execErr = engine.Query(sql.NewEmptyContext(), query)
			if execErr == nil {
				execErr = drainIter(rowIter)
			}
		case *sqlparser.DDL:
			var rowIter sql.RowIter
			_, rowIter, execErr = engine.Query(sql.NewEmptyContext(), query)
			if execErr == nil {
				execErr = drainIter(rowIter)
			}
			if err = db.Flush(context.Background()); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("Unsupported SQL statement: '%v'.", query)
		}

		if execErr != nil {
			return nil, execErr
		}
	}

	if err := db.Flush(context.Background()); err == nil {
		return db.Root(), nil
	} else {
		return nil, err
	}
}

// Executes the select statement given and returns the resulting rows, or an error if one is encountered.
// This uses the index functionality, which is not ready for prime time. Use with caution.
func ExecuteSelect(root *doltdb.RootValue, query string) ([]sql.Row, error) {
	db := NewDatabase("dolt", root, nil, nil)
	engine := sqle.NewDefault()
	engine.AddDatabase(db)
	engine.Catalog.RegisterIndexDriver(NewDoltIndexDriver(db))
	_ = engine.Init()

	ctx := sql.NewEmptyContext()
	_, rowIter, err := engine.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	var (
		rows   []sql.Row
		rowErr error
		row    sql.Row
	)
	for row, rowErr = rowIter.Next(); rowErr == nil; row, rowErr = rowIter.Next() {
		rows = append(rows, row)
	}

	if rowErr != io.EOF {
		return nil, rowErr
	}

	return rows, nil
}

func drainIter(iter sql.RowIter) error {
	var returnedErr error
	for {
		_, err := iter.Next()
		if err == io.EOF {
			return returnedErr
		} else if err != nil {
			returnedErr = err
		}
	}
}
