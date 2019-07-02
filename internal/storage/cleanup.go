package db

import (
	"fmt"
	"strings"

	"github.com/volatiletech/sqlboiler/queries"
)

var tables = []string{
	"users",
}

func Cleanup(c ConnData) {
	queries.Raw(fmt.Sprintf("TRUNCATE %s CASCADE;", strings.Join(tables, ", "))).Exec(c.DB)
}
