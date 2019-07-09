package storage

import (
	"fmt"
	"strings"

	"github.com/volatiletech/sqlboiler/queries"
)


func Cleanup(c ConnData, tables []string) {
	queries.Raw(fmt.Sprintf("TRUNCATE %s CASCADE;", strings.Join(tables, ", "))).Exec(c.DB)
}
