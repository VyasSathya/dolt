// Copyright 2020 Dolthub, Inc.
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

package dfunctions

import (
	"fmt"
	"github.com/dolthub/dolt/go/cmd/dolt/cli"
	"github.com/dolthub/dolt/go/libraries/doltcore/env/actions"
	"github.com/dolthub/dolt/go/libraries/doltcore/sqle"
	"github.com/dolthub/go-mysql-server/sql"
)

const DoltCommitFuncName = "dolt_commit"

type DoltCommitFunc struct {
	children []sql.Expression
}

// NewDoltCommitFunc creates a new DoltCommitFunc expression.
func NewDoltCommitFunc(args ...sql.Expression) (sql.Expression, error) {
	return &DoltCommitFunc{children: args}, nil
}

// Trims the double quotes for the param.
func trimQuotes(param string) string {
	if len(param) > 0 && param[0] == '"' {
		param = param[1:]
	}

	if len(param) > 0 && param[len(param)-1] == '"' {
		param = param[:len(param)-1]
	}

	return param
}

func (d DoltCommitFunc) Eval(ctx *sql.Context, row sql.Row) (interface{}, error) {
	// Get the information for the sql context.
	dbName := ctx.GetCurrentDatabase()
	dSess := sqle.DSessFromSess(ctx.Session)

	ddb, ok := dSess.GetDoltDB(dbName)

	if !ok {
		return nil, fmt.Errorf("Could not load %s", dbName)
	}

	rsr, ok := dSess.GetDoltDBRepoStateReader(dbName)

	if !ok {
		return nil, fmt.Errorf("Could not load the %s RepoStateReader", dbName)
	}

	rsw, ok := dSess.GetDoltDBRepoStateWriter(dbName)

	if !ok {
		return nil, fmt.Errorf("Could not load the %s RepoStateWriter", dbName)
	}

	ap := actions.CreateCommitArgParser()

	// Get the args for DOLT_COMMIT.
	args := make([]string, 0)
	for i := range d.children {
		eval, err := d.children[i].Eval(ctx, row)

		if err != nil {
			return "", err
		}

		eval, err = sql.Text.Convert(eval)

		if eval != nil {
			return "", nil
		}

		str := trimQuotes(string.(eval))

		args = append(args, str)
	}

	apr := cli.ParseArgs(ap, args, nil)

	// Parse the author flag. Return an error if not.
	var name, email string
	var err error
	if authorStr, ok := apr.GetValue(actions.AuthorParam); ok {
		name, email, err = actions.ParseAuthor(authorStr)
		if err != nil {
			return nil, err
		}
	} else {
		name = dSess.Username
		email = dSess.Email
	}

	// Get the commit message.
	msg, msgOk := apr.GetValue(actions.CommitMessageArg)
	if !msgOk {
		return nil, fmt.Errorf("Must provide commit message.")
	}

	// Specify the time if the date parameter is not.
	t := ctx.QueryTime()
	if commitTimeStr, ok := apr.GetValue(actions.DateParam); ok {
		var err error
		t, err = actions.ParseDate(commitTimeStr)

		if err != nil {
			return nil, fmt.Errorf(err.Error())
		}
	}

	h, err := actions.CommitStaged(ctx, ddb, rsr, rsw, actions.CommitStagedProps{
		Message:          msg,
		Date:             t,
		AllowEmpty:       apr.Contains(actions.AllowEmptyFlag),
		CheckForeignKeys: !apr.Contains(actions.ForceFlag),
		Name:             name,
		Email:            email,
	})

	return h, err
}

func (d DoltCommitFunc) String() string {
	childrenStrings := make([]string, len(d.children))

	for _, child := range d.children {
		childrenStrings = append(childrenStrings, child.String())
	}
	return fmt.Sprintf("commit_hash")
}

func (d DoltCommitFunc) Type() sql.Type {
	return sql.Text
}

func (d DoltCommitFunc) IsNullable() bool {
	return false
}

func (d DoltCommitFunc) WithChildren(children ...sql.Expression) (sql.Expression, error) {
	return NewDoltCommitFunc(children...)
}

func (d DoltCommitFunc) Resolved() bool {
	for _, child := range d.Children() {
		if !child.Resolved() {
			return false
		}
	}
	return true
}

func (d DoltCommitFunc) Children() []sql.Expression {
	return d.children
}
