// Code generated by SQLBoiler (https://github.com/volatiletech/sqlboiler). DO NOT EDIT.
// This file is meant to be re-generated in place and/or deleted at any time.

package models

import (
	"database/sql"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/volatiletech/null"
	"github.com/volatiletech/sqlboiler/boil"
	"github.com/volatiletech/sqlboiler/queries"
	"github.com/volatiletech/sqlboiler/queries/qm"
	"github.com/volatiletech/sqlboiler/queries/qmhelper"
	"github.com/volatiletech/sqlboiler/strmangle"
)

// QueryLog is an object representing the database table.
type QueryLog struct {
	ID        int       `boil:"id" json:"id" toml:"id" yaml:"id"`
	Method    string    `boil:"method" json:"method" toml:"method" yaml:"method"`
	Timestamp time.Time `boil:"timestamp" json:"timestamp" toml:"timestamp" yaml:"timestamp"`
	UserID    null.Int  `boil:"user_id" json:"user_id,omitempty" toml:"user_id" yaml:"user_id,omitempty"`
	RemoteIP  string    `boil:"remote_ip" json:"remote_ip" toml:"remote_ip" yaml:"remote_ip"`
	Body      null.JSON `boil:"body" json:"body,omitempty" toml:"body" yaml:"body,omitempty"`

	R *queryLogR `boil:"-" json:"-" toml:"-" yaml:"-"`
	L queryLogL  `boil:"-" json:"-" toml:"-" yaml:"-"`
}

var QueryLogColumns = struct {
	ID        string
	Method    string
	Timestamp string
	UserID    string
	RemoteIP  string
	Body      string
}{
	ID:        "id",
	Method:    "method",
	Timestamp: "timestamp",
	UserID:    "user_id",
	RemoteIP:  "remote_ip",
	Body:      "body",
}

// Generated where

var QueryLogWhere = struct {
	ID        whereHelperint
	Method    whereHelperstring
	Timestamp whereHelpertime_Time
	UserID    whereHelpernull_Int
	RemoteIP  whereHelperstring
	Body      whereHelpernull_JSON
}{
	ID:        whereHelperint{field: "\"query_log\".\"id\""},
	Method:    whereHelperstring{field: "\"query_log\".\"method\""},
	Timestamp: whereHelpertime_Time{field: "\"query_log\".\"timestamp\""},
	UserID:    whereHelpernull_Int{field: "\"query_log\".\"user_id\""},
	RemoteIP:  whereHelperstring{field: "\"query_log\".\"remote_ip\""},
	Body:      whereHelpernull_JSON{field: "\"query_log\".\"body\""},
}

// QueryLogRels is where relationship names are stored.
var QueryLogRels = struct {
}{}

// queryLogR is where relationships are stored.
type queryLogR struct {
}

// NewStruct creates a new relationship struct
func (*queryLogR) NewStruct() *queryLogR {
	return &queryLogR{}
}

// queryLogL is where Load methods for each relationship are stored.
type queryLogL struct{}

var (
	queryLogAllColumns            = []string{"id", "method", "timestamp", "user_id", "remote_ip", "body"}
	queryLogColumnsWithoutDefault = []string{"method", "user_id", "remote_ip", "body"}
	queryLogColumnsWithDefault    = []string{"id", "timestamp"}
	queryLogPrimaryKeyColumns     = []string{"id"}
)

type (
	// QueryLogSlice is an alias for a slice of pointers to QueryLog.
	// This should generally be used opposed to []QueryLog.
	QueryLogSlice []*QueryLog
	// QueryLogHook is the signature for custom QueryLog hook methods
	QueryLogHook func(boil.Executor, *QueryLog) error

	queryLogQuery struct {
		*queries.Query
	}
)

// Cache for insert, update and upsert
var (
	queryLogType                 = reflect.TypeOf(&QueryLog{})
	queryLogMapping              = queries.MakeStructMapping(queryLogType)
	queryLogPrimaryKeyMapping, _ = queries.BindMapping(queryLogType, queryLogMapping, queryLogPrimaryKeyColumns)
	queryLogInsertCacheMut       sync.RWMutex
	queryLogInsertCache          = make(map[string]insertCache)
	queryLogUpdateCacheMut       sync.RWMutex
	queryLogUpdateCache          = make(map[string]updateCache)
	queryLogUpsertCacheMut       sync.RWMutex
	queryLogUpsertCache          = make(map[string]insertCache)
)

var (
	// Force time package dependency for automated UpdatedAt/CreatedAt.
	_ = time.Second
	// Force qmhelper dependency for where clause generation (which doesn't
	// always happen)
	_ = qmhelper.Where
)

var queryLogBeforeInsertHooks []QueryLogHook
var queryLogBeforeUpdateHooks []QueryLogHook
var queryLogBeforeDeleteHooks []QueryLogHook
var queryLogBeforeUpsertHooks []QueryLogHook

var queryLogAfterInsertHooks []QueryLogHook
var queryLogAfterSelectHooks []QueryLogHook
var queryLogAfterUpdateHooks []QueryLogHook
var queryLogAfterDeleteHooks []QueryLogHook
var queryLogAfterUpsertHooks []QueryLogHook

// doBeforeInsertHooks executes all "before insert" hooks.
func (o *QueryLog) doBeforeInsertHooks(exec boil.Executor) (err error) {
	for _, hook := range queryLogBeforeInsertHooks {
		if err := hook(exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doBeforeUpdateHooks executes all "before Update" hooks.
func (o *QueryLog) doBeforeUpdateHooks(exec boil.Executor) (err error) {
	for _, hook := range queryLogBeforeUpdateHooks {
		if err := hook(exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doBeforeDeleteHooks executes all "before Delete" hooks.
func (o *QueryLog) doBeforeDeleteHooks(exec boil.Executor) (err error) {
	for _, hook := range queryLogBeforeDeleteHooks {
		if err := hook(exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doBeforeUpsertHooks executes all "before Upsert" hooks.
func (o *QueryLog) doBeforeUpsertHooks(exec boil.Executor) (err error) {
	for _, hook := range queryLogBeforeUpsertHooks {
		if err := hook(exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doAfterInsertHooks executes all "after Insert" hooks.
func (o *QueryLog) doAfterInsertHooks(exec boil.Executor) (err error) {
	for _, hook := range queryLogAfterInsertHooks {
		if err := hook(exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doAfterSelectHooks executes all "after Select" hooks.
func (o *QueryLog) doAfterSelectHooks(exec boil.Executor) (err error) {
	for _, hook := range queryLogAfterSelectHooks {
		if err := hook(exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doAfterUpdateHooks executes all "after Update" hooks.
func (o *QueryLog) doAfterUpdateHooks(exec boil.Executor) (err error) {
	for _, hook := range queryLogAfterUpdateHooks {
		if err := hook(exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doAfterDeleteHooks executes all "after Delete" hooks.
func (o *QueryLog) doAfterDeleteHooks(exec boil.Executor) (err error) {
	for _, hook := range queryLogAfterDeleteHooks {
		if err := hook(exec, o); err != nil {
			return err
		}
	}

	return nil
}

// doAfterUpsertHooks executes all "after Upsert" hooks.
func (o *QueryLog) doAfterUpsertHooks(exec boil.Executor) (err error) {
	for _, hook := range queryLogAfterUpsertHooks {
		if err := hook(exec, o); err != nil {
			return err
		}
	}

	return nil
}

// AddQueryLogHook registers your hook function for all future operations.
func AddQueryLogHook(hookPoint boil.HookPoint, queryLogHook QueryLogHook) {
	switch hookPoint {
	case boil.BeforeInsertHook:
		queryLogBeforeInsertHooks = append(queryLogBeforeInsertHooks, queryLogHook)
	case boil.BeforeUpdateHook:
		queryLogBeforeUpdateHooks = append(queryLogBeforeUpdateHooks, queryLogHook)
	case boil.BeforeDeleteHook:
		queryLogBeforeDeleteHooks = append(queryLogBeforeDeleteHooks, queryLogHook)
	case boil.BeforeUpsertHook:
		queryLogBeforeUpsertHooks = append(queryLogBeforeUpsertHooks, queryLogHook)
	case boil.AfterInsertHook:
		queryLogAfterInsertHooks = append(queryLogAfterInsertHooks, queryLogHook)
	case boil.AfterSelectHook:
		queryLogAfterSelectHooks = append(queryLogAfterSelectHooks, queryLogHook)
	case boil.AfterUpdateHook:
		queryLogAfterUpdateHooks = append(queryLogAfterUpdateHooks, queryLogHook)
	case boil.AfterDeleteHook:
		queryLogAfterDeleteHooks = append(queryLogAfterDeleteHooks, queryLogHook)
	case boil.AfterUpsertHook:
		queryLogAfterUpsertHooks = append(queryLogAfterUpsertHooks, queryLogHook)
	}
}

// OneG returns a single queryLog record from the query using the global executor.
func (q queryLogQuery) OneG() (*QueryLog, error) {
	return q.One(boil.GetDB())
}

// One returns a single queryLog record from the query.
func (q queryLogQuery) One(exec boil.Executor) (*QueryLog, error) {
	o := &QueryLog{}

	queries.SetLimit(q.Query, 1)

	err := q.Bind(nil, exec, o)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "models: failed to execute a one query for query_log")
	}

	if err := o.doAfterSelectHooks(exec); err != nil {
		return o, err
	}

	return o, nil
}

// AllG returns all QueryLog records from the query using the global executor.
func (q queryLogQuery) AllG() (QueryLogSlice, error) {
	return q.All(boil.GetDB())
}

// All returns all QueryLog records from the query.
func (q queryLogQuery) All(exec boil.Executor) (QueryLogSlice, error) {
	var o []*QueryLog

	err := q.Bind(nil, exec, &o)
	if err != nil {
		return nil, errors.Wrap(err, "models: failed to assign all query results to QueryLog slice")
	}

	if len(queryLogAfterSelectHooks) != 0 {
		for _, obj := range o {
			if err := obj.doAfterSelectHooks(exec); err != nil {
				return o, err
			}
		}
	}

	return o, nil
}

// CountG returns the count of all QueryLog records in the query, and panics on error.
func (q queryLogQuery) CountG() (int64, error) {
	return q.Count(boil.GetDB())
}

// Count returns the count of all QueryLog records in the query.
func (q queryLogQuery) Count(exec boil.Executor) (int64, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)

	err := q.Query.QueryRow(exec).Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to count query_log rows")
	}

	return count, nil
}

// ExistsG checks if the row exists in the table, and panics on error.
func (q queryLogQuery) ExistsG() (bool, error) {
	return q.Exists(boil.GetDB())
}

// Exists checks if the row exists in the table.
func (q queryLogQuery) Exists(exec boil.Executor) (bool, error) {
	var count int64

	queries.SetSelect(q.Query, nil)
	queries.SetCount(q.Query)
	queries.SetLimit(q.Query, 1)

	err := q.Query.QueryRow(exec).Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, "models: failed to check if query_log exists")
	}

	return count > 0, nil
}

// QueryLogs retrieves all the records using an executor.
func QueryLogs(mods ...qm.QueryMod) queryLogQuery {
	mods = append(mods, qm.From("\"query_log\""))
	return queryLogQuery{NewQuery(mods...)}
}

// FindQueryLogG retrieves a single record by ID.
func FindQueryLogG(iD int, selectCols ...string) (*QueryLog, error) {
	return FindQueryLog(boil.GetDB(), iD, selectCols...)
}

// FindQueryLog retrieves a single record by ID with an executor.
// If selectCols is empty Find will return all columns.
func FindQueryLog(exec boil.Executor, iD int, selectCols ...string) (*QueryLog, error) {
	queryLogObj := &QueryLog{}

	sel := "*"
	if len(selectCols) > 0 {
		sel = strings.Join(strmangle.IdentQuoteSlice(dialect.LQ, dialect.RQ, selectCols), ",")
	}
	query := fmt.Sprintf(
		"select %s from \"query_log\" where \"id\"=$1", sel,
	)

	q := queries.Raw(query, iD)

	err := q.Bind(nil, exec, queryLogObj)
	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, errors.Wrap(err, "models: unable to select from query_log")
	}

	return queryLogObj, nil
}

// InsertG a single record. See Insert for whitelist behavior description.
func (o *QueryLog) InsertG(columns boil.Columns) error {
	return o.Insert(boil.GetDB(), columns)
}

// Insert a single record using an executor.
// See boil.Columns.InsertColumnSet documentation to understand column list inference for inserts.
func (o *QueryLog) Insert(exec boil.Executor, columns boil.Columns) error {
	if o == nil {
		return errors.New("models: no query_log provided for insertion")
	}

	var err error

	if err := o.doBeforeInsertHooks(exec); err != nil {
		return err
	}

	nzDefaults := queries.NonZeroDefaultSet(queryLogColumnsWithDefault, o)

	key := makeCacheKey(columns, nzDefaults)
	queryLogInsertCacheMut.RLock()
	cache, cached := queryLogInsertCache[key]
	queryLogInsertCacheMut.RUnlock()

	if !cached {
		wl, returnColumns := columns.InsertColumnSet(
			queryLogAllColumns,
			queryLogColumnsWithDefault,
			queryLogColumnsWithoutDefault,
			nzDefaults,
		)

		cache.valueMapping, err = queries.BindMapping(queryLogType, queryLogMapping, wl)
		if err != nil {
			return err
		}
		cache.retMapping, err = queries.BindMapping(queryLogType, queryLogMapping, returnColumns)
		if err != nil {
			return err
		}
		if len(wl) != 0 {
			cache.query = fmt.Sprintf("INSERT INTO \"query_log\" (\"%s\") %%sVALUES (%s)%%s", strings.Join(wl, "\",\""), strmangle.Placeholders(dialect.UseIndexPlaceholders, len(wl), 1, 1))
		} else {
			cache.query = "INSERT INTO \"query_log\" %sDEFAULT VALUES%s"
		}

		var queryOutput, queryReturning string

		if len(cache.retMapping) != 0 {
			queryReturning = fmt.Sprintf(" RETURNING \"%s\"", strings.Join(returnColumns, "\",\""))
		}

		cache.query = fmt.Sprintf(cache.query, queryOutput, queryReturning)
	}

	value := reflect.Indirect(reflect.ValueOf(o))
	vals := queries.ValuesFromMapping(value, cache.valueMapping)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, cache.query)
		fmt.Fprintln(boil.DebugWriter, vals)
	}

	if len(cache.retMapping) != 0 {
		err = exec.QueryRow(cache.query, vals...).Scan(queries.PtrsFromMapping(value, cache.retMapping)...)
	} else {
		_, err = exec.Exec(cache.query, vals...)
	}

	if err != nil {
		return errors.Wrap(err, "models: unable to insert into query_log")
	}

	if !cached {
		queryLogInsertCacheMut.Lock()
		queryLogInsertCache[key] = cache
		queryLogInsertCacheMut.Unlock()
	}

	return o.doAfterInsertHooks(exec)
}

// UpdateG a single QueryLog record using the global executor.
// See Update for more documentation.
func (o *QueryLog) UpdateG(columns boil.Columns) (int64, error) {
	return o.Update(boil.GetDB(), columns)
}

// Update uses an executor to update the QueryLog.
// See boil.Columns.UpdateColumnSet documentation to understand column list inference for updates.
// Update does not automatically update the record in case of default values. Use .Reload() to refresh the records.
func (o *QueryLog) Update(exec boil.Executor, columns boil.Columns) (int64, error) {
	var err error
	if err = o.doBeforeUpdateHooks(exec); err != nil {
		return 0, err
	}
	key := makeCacheKey(columns, nil)
	queryLogUpdateCacheMut.RLock()
	cache, cached := queryLogUpdateCache[key]
	queryLogUpdateCacheMut.RUnlock()

	if !cached {
		wl := columns.UpdateColumnSet(
			queryLogAllColumns,
			queryLogPrimaryKeyColumns,
		)

		if !columns.IsWhitelist() {
			wl = strmangle.SetComplement(wl, []string{"created_at"})
		}
		if len(wl) == 0 {
			return 0, errors.New("models: unable to update query_log, could not build whitelist")
		}

		cache.query = fmt.Sprintf("UPDATE \"query_log\" SET %s WHERE %s",
			strmangle.SetParamNames("\"", "\"", 1, wl),
			strmangle.WhereClause("\"", "\"", len(wl)+1, queryLogPrimaryKeyColumns),
		)
		cache.valueMapping, err = queries.BindMapping(queryLogType, queryLogMapping, append(wl, queryLogPrimaryKeyColumns...))
		if err != nil {
			return 0, err
		}
	}

	values := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), cache.valueMapping)

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, cache.query)
		fmt.Fprintln(boil.DebugWriter, values)
	}

	var result sql.Result
	result, err = exec.Exec(cache.query, values...)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to update query_log row")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to get rows affected by update for query_log")
	}

	if !cached {
		queryLogUpdateCacheMut.Lock()
		queryLogUpdateCache[key] = cache
		queryLogUpdateCacheMut.Unlock()
	}

	return rowsAff, o.doAfterUpdateHooks(exec)
}

// UpdateAllG updates all rows with the specified column values.
func (q queryLogQuery) UpdateAllG(cols M) (int64, error) {
	return q.UpdateAll(boil.GetDB(), cols)
}

// UpdateAll updates all rows with the specified column values.
func (q queryLogQuery) UpdateAll(exec boil.Executor, cols M) (int64, error) {
	queries.SetUpdate(q.Query, cols)

	result, err := q.Query.Exec(exec)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to update all for query_log")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to retrieve rows affected for query_log")
	}

	return rowsAff, nil
}

// UpdateAllG updates all rows with the specified column values.
func (o QueryLogSlice) UpdateAllG(cols M) (int64, error) {
	return o.UpdateAll(boil.GetDB(), cols)
}

// UpdateAll updates all rows with the specified column values, using an executor.
func (o QueryLogSlice) UpdateAll(exec boil.Executor, cols M) (int64, error) {
	ln := int64(len(o))
	if ln == 0 {
		return 0, nil
	}

	if len(cols) == 0 {
		return 0, errors.New("models: update all requires at least one column argument")
	}

	colNames := make([]string, len(cols))
	args := make([]interface{}, len(cols))

	i := 0
	for name, value := range cols {
		colNames[i] = name
		args[i] = value
		i++
	}

	// Append all of the primary key values for each column
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), queryLogPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := fmt.Sprintf("UPDATE \"query_log\" SET %s WHERE %s",
		strmangle.SetParamNames("\"", "\"", 1, colNames),
		strmangle.WhereClauseRepeated(string(dialect.LQ), string(dialect.RQ), len(colNames)+1, queryLogPrimaryKeyColumns, len(o)))

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	result, err := exec.Exec(sql, args...)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to update all in queryLog slice")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to retrieve rows affected all in update all queryLog")
	}
	return rowsAff, nil
}

// UpsertG attempts an insert, and does an update or ignore on conflict.
func (o *QueryLog) UpsertG(updateOnConflict bool, conflictColumns []string, updateColumns, insertColumns boil.Columns) error {
	return o.Upsert(boil.GetDB(), updateOnConflict, conflictColumns, updateColumns, insertColumns)
}

// Upsert attempts an insert using an executor, and does an update or ignore on conflict.
// See boil.Columns documentation for how to properly use updateColumns and insertColumns.
func (o *QueryLog) Upsert(exec boil.Executor, updateOnConflict bool, conflictColumns []string, updateColumns, insertColumns boil.Columns) error {
	if o == nil {
		return errors.New("models: no query_log provided for upsert")
	}

	if err := o.doBeforeUpsertHooks(exec); err != nil {
		return err
	}

	nzDefaults := queries.NonZeroDefaultSet(queryLogColumnsWithDefault, o)

	// Build cache key in-line uglily - mysql vs psql problems
	buf := strmangle.GetBuffer()
	if updateOnConflict {
		buf.WriteByte('t')
	} else {
		buf.WriteByte('f')
	}
	buf.WriteByte('.')
	for _, c := range conflictColumns {
		buf.WriteString(c)
	}
	buf.WriteByte('.')
	buf.WriteString(strconv.Itoa(updateColumns.Kind))
	for _, c := range updateColumns.Cols {
		buf.WriteString(c)
	}
	buf.WriteByte('.')
	buf.WriteString(strconv.Itoa(insertColumns.Kind))
	for _, c := range insertColumns.Cols {
		buf.WriteString(c)
	}
	buf.WriteByte('.')
	for _, c := range nzDefaults {
		buf.WriteString(c)
	}
	key := buf.String()
	strmangle.PutBuffer(buf)

	queryLogUpsertCacheMut.RLock()
	cache, cached := queryLogUpsertCache[key]
	queryLogUpsertCacheMut.RUnlock()

	var err error

	if !cached {
		insert, ret := insertColumns.InsertColumnSet(
			queryLogAllColumns,
			queryLogColumnsWithDefault,
			queryLogColumnsWithoutDefault,
			nzDefaults,
		)
		update := updateColumns.UpdateColumnSet(
			queryLogAllColumns,
			queryLogPrimaryKeyColumns,
		)

		if updateOnConflict && len(update) == 0 {
			return errors.New("models: unable to upsert query_log, could not build update column list")
		}

		conflict := conflictColumns
		if len(conflict) == 0 {
			conflict = make([]string, len(queryLogPrimaryKeyColumns))
			copy(conflict, queryLogPrimaryKeyColumns)
		}
		cache.query = buildUpsertQueryPostgres(dialect, "\"query_log\"", updateOnConflict, ret, update, conflict, insert)

		cache.valueMapping, err = queries.BindMapping(queryLogType, queryLogMapping, insert)
		if err != nil {
			return err
		}
		if len(ret) != 0 {
			cache.retMapping, err = queries.BindMapping(queryLogType, queryLogMapping, ret)
			if err != nil {
				return err
			}
		}
	}

	value := reflect.Indirect(reflect.ValueOf(o))
	vals := queries.ValuesFromMapping(value, cache.valueMapping)
	var returns []interface{}
	if len(cache.retMapping) != 0 {
		returns = queries.PtrsFromMapping(value, cache.retMapping)
	}

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, cache.query)
		fmt.Fprintln(boil.DebugWriter, vals)
	}

	if len(cache.retMapping) != 0 {
		err = exec.QueryRow(cache.query, vals...).Scan(returns...)
		if err == sql.ErrNoRows {
			err = nil // Postgres doesn't return anything when there's no update
		}
	} else {
		_, err = exec.Exec(cache.query, vals...)
	}
	if err != nil {
		return errors.Wrap(err, "models: unable to upsert query_log")
	}

	if !cached {
		queryLogUpsertCacheMut.Lock()
		queryLogUpsertCache[key] = cache
		queryLogUpsertCacheMut.Unlock()
	}

	return o.doAfterUpsertHooks(exec)
}

// DeleteG deletes a single QueryLog record.
// DeleteG will match against the primary key column to find the record to delete.
func (o *QueryLog) DeleteG() (int64, error) {
	return o.Delete(boil.GetDB())
}

// Delete deletes a single QueryLog record with an executor.
// Delete will match against the primary key column to find the record to delete.
func (o *QueryLog) Delete(exec boil.Executor) (int64, error) {
	if o == nil {
		return 0, errors.New("models: no QueryLog provided for delete")
	}

	if err := o.doBeforeDeleteHooks(exec); err != nil {
		return 0, err
	}

	args := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(o)), queryLogPrimaryKeyMapping)
	sql := "DELETE FROM \"query_log\" WHERE \"id\"=$1"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args...)
	}

	result, err := exec.Exec(sql, args...)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to delete from query_log")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to get rows affected by delete for query_log")
	}

	if err := o.doAfterDeleteHooks(exec); err != nil {
		return 0, err
	}

	return rowsAff, nil
}

// DeleteAll deletes all matching rows.
func (q queryLogQuery) DeleteAll(exec boil.Executor) (int64, error) {
	if q.Query == nil {
		return 0, errors.New("models: no queryLogQuery provided for delete all")
	}

	queries.SetDelete(q.Query)

	result, err := q.Query.Exec(exec)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to delete all from query_log")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to get rows affected by deleteall for query_log")
	}

	return rowsAff, nil
}

// DeleteAllG deletes all rows in the slice.
func (o QueryLogSlice) DeleteAllG() (int64, error) {
	return o.DeleteAll(boil.GetDB())
}

// DeleteAll deletes all rows in the slice, using an executor.
func (o QueryLogSlice) DeleteAll(exec boil.Executor) (int64, error) {
	if len(o) == 0 {
		return 0, nil
	}

	if len(queryLogBeforeDeleteHooks) != 0 {
		for _, obj := range o {
			if err := obj.doBeforeDeleteHooks(exec); err != nil {
				return 0, err
			}
		}
	}

	var args []interface{}
	for _, obj := range o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), queryLogPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := "DELETE FROM \"query_log\" WHERE " +
		strmangle.WhereClauseRepeated(string(dialect.LQ), string(dialect.RQ), 1, queryLogPrimaryKeyColumns, len(o))

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, args)
	}

	result, err := exec.Exec(sql, args...)
	if err != nil {
		return 0, errors.Wrap(err, "models: unable to delete all from queryLog slice")
	}

	rowsAff, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "models: failed to get rows affected by deleteall for query_log")
	}

	if len(queryLogAfterDeleteHooks) != 0 {
		for _, obj := range o {
			if err := obj.doAfterDeleteHooks(exec); err != nil {
				return 0, err
			}
		}
	}

	return rowsAff, nil
}

// ReloadG refetches the object from the database using the primary keys.
func (o *QueryLog) ReloadG() error {
	if o == nil {
		return errors.New("models: no QueryLog provided for reload")
	}

	return o.Reload(boil.GetDB())
}

// Reload refetches the object from the database
// using the primary keys with an executor.
func (o *QueryLog) Reload(exec boil.Executor) error {
	ret, err := FindQueryLog(exec, o.ID)
	if err != nil {
		return err
	}

	*o = *ret
	return nil
}

// ReloadAllG refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *QueryLogSlice) ReloadAllG() error {
	if o == nil {
		return errors.New("models: empty QueryLogSlice provided for reload all")
	}

	return o.ReloadAll(boil.GetDB())
}

// ReloadAll refetches every row with matching primary key column values
// and overwrites the original object slice with the newly updated slice.
func (o *QueryLogSlice) ReloadAll(exec boil.Executor) error {
	if o == nil || len(*o) == 0 {
		return nil
	}

	slice := QueryLogSlice{}
	var args []interface{}
	for _, obj := range *o {
		pkeyArgs := queries.ValuesFromMapping(reflect.Indirect(reflect.ValueOf(obj)), queryLogPrimaryKeyMapping)
		args = append(args, pkeyArgs...)
	}

	sql := "SELECT \"query_log\".* FROM \"query_log\" WHERE " +
		strmangle.WhereClauseRepeated(string(dialect.LQ), string(dialect.RQ), 1, queryLogPrimaryKeyColumns, len(*o))

	q := queries.Raw(sql, args...)

	err := q.Bind(nil, exec, &slice)
	if err != nil {
		return errors.Wrap(err, "models: unable to reload all in QueryLogSlice")
	}

	*o = slice

	return nil
}

// QueryLogExistsG checks if the QueryLog row exists.
func QueryLogExistsG(iD int) (bool, error) {
	return QueryLogExists(boil.GetDB(), iD)
}

// QueryLogExists checks if the QueryLog row exists.
func QueryLogExists(exec boil.Executor, iD int) (bool, error) {
	var exists bool
	sql := "select exists(select 1 from \"query_log\" where \"id\"=$1 limit 1)"

	if boil.DebugMode {
		fmt.Fprintln(boil.DebugWriter, sql)
		fmt.Fprintln(boil.DebugWriter, iD)
	}

	row := exec.QueryRow(sql, iD)

	err := row.Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, "models: unable to check if query_log exists")
	}

	return exists, nil
}
