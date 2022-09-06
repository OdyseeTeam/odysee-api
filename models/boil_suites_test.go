// Code generated by SQLBoiler (https://github.com/volatiletech/sqlboiler). DO NOT EDIT.
// This file is meant to be re-generated in place and/or deleted at any time.

package models

import "testing"

// This test suite runs each operation test in parallel.
// Example, if your database has 3 tables, the suite will run:
// table1, table2 and table3 Delete in parallel
// table1, table2 and table3 Insert in parallel, and so forth.
// It does NOT run each operation group in parallel.
// Separating the tests thusly grants avoidance of Postgres deadlocks.
func TestParent(t *testing.T) {
	t.Run("GorpMigrations", testGorpMigrations)
	t.Run("LbrynetServers", testLbrynetServers)
	t.Run("PublishQueries", testPublishQueries)
	t.Run("QueryLogs", testQueryLogs)
	t.Run("Uploads", testUploads)
	t.Run("Users", testUsers)
}

func TestDelete(t *testing.T) {
	t.Run("GorpMigrations", testGorpMigrationsDelete)
	t.Run("LbrynetServers", testLbrynetServersDelete)
	t.Run("PublishQueries", testPublishQueriesDelete)
	t.Run("QueryLogs", testQueryLogsDelete)
	t.Run("Uploads", testUploadsDelete)
	t.Run("Users", testUsersDelete)
}

func TestQueryDeleteAll(t *testing.T) {
	t.Run("GorpMigrations", testGorpMigrationsQueryDeleteAll)
	t.Run("LbrynetServers", testLbrynetServersQueryDeleteAll)
	t.Run("PublishQueries", testPublishQueriesQueryDeleteAll)
	t.Run("QueryLogs", testQueryLogsQueryDeleteAll)
	t.Run("Uploads", testUploadsQueryDeleteAll)
	t.Run("Users", testUsersQueryDeleteAll)
}

func TestSliceDeleteAll(t *testing.T) {
	t.Run("GorpMigrations", testGorpMigrationsSliceDeleteAll)
	t.Run("LbrynetServers", testLbrynetServersSliceDeleteAll)
	t.Run("PublishQueries", testPublishQueriesSliceDeleteAll)
	t.Run("QueryLogs", testQueryLogsSliceDeleteAll)
	t.Run("Uploads", testUploadsSliceDeleteAll)
	t.Run("Users", testUsersSliceDeleteAll)
}

func TestExists(t *testing.T) {
	t.Run("GorpMigrations", testGorpMigrationsExists)
	t.Run("LbrynetServers", testLbrynetServersExists)
	t.Run("PublishQueries", testPublishQueriesExists)
	t.Run("QueryLogs", testQueryLogsExists)
	t.Run("Uploads", testUploadsExists)
	t.Run("Users", testUsersExists)
}

func TestFind(t *testing.T) {
	t.Run("GorpMigrations", testGorpMigrationsFind)
	t.Run("LbrynetServers", testLbrynetServersFind)
	t.Run("PublishQueries", testPublishQueriesFind)
	t.Run("QueryLogs", testQueryLogsFind)
	t.Run("Uploads", testUploadsFind)
	t.Run("Users", testUsersFind)
}

func TestBind(t *testing.T) {
	t.Run("GorpMigrations", testGorpMigrationsBind)
	t.Run("LbrynetServers", testLbrynetServersBind)
	t.Run("PublishQueries", testPublishQueriesBind)
	t.Run("QueryLogs", testQueryLogsBind)
	t.Run("Uploads", testUploadsBind)
	t.Run("Users", testUsersBind)
}

func TestOne(t *testing.T) {
	t.Run("GorpMigrations", testGorpMigrationsOne)
	t.Run("LbrynetServers", testLbrynetServersOne)
	t.Run("PublishQueries", testPublishQueriesOne)
	t.Run("QueryLogs", testQueryLogsOne)
	t.Run("Uploads", testUploadsOne)
	t.Run("Users", testUsersOne)
}

func TestAll(t *testing.T) {
	t.Run("GorpMigrations", testGorpMigrationsAll)
	t.Run("LbrynetServers", testLbrynetServersAll)
	t.Run("PublishQueries", testPublishQueriesAll)
	t.Run("QueryLogs", testQueryLogsAll)
	t.Run("Uploads", testUploadsAll)
	t.Run("Users", testUsersAll)
}

func TestCount(t *testing.T) {
	t.Run("GorpMigrations", testGorpMigrationsCount)
	t.Run("LbrynetServers", testLbrynetServersCount)
	t.Run("PublishQueries", testPublishQueriesCount)
	t.Run("QueryLogs", testQueryLogsCount)
	t.Run("Uploads", testUploadsCount)
	t.Run("Users", testUsersCount)
}

func TestHooks(t *testing.T) {
	t.Run("GorpMigrations", testGorpMigrationsHooks)
	t.Run("LbrynetServers", testLbrynetServersHooks)
	t.Run("PublishQueries", testPublishQueriesHooks)
	t.Run("QueryLogs", testQueryLogsHooks)
	t.Run("Uploads", testUploadsHooks)
	t.Run("Users", testUsersHooks)
}

func TestInsert(t *testing.T) {
	t.Run("GorpMigrations", testGorpMigrationsInsert)
	t.Run("GorpMigrations", testGorpMigrationsInsertWhitelist)
	t.Run("LbrynetServers", testLbrynetServersInsert)
	t.Run("LbrynetServers", testLbrynetServersInsertWhitelist)
	t.Run("PublishQueries", testPublishQueriesInsert)
	t.Run("PublishQueries", testPublishQueriesInsertWhitelist)
	t.Run("QueryLogs", testQueryLogsInsert)
	t.Run("QueryLogs", testQueryLogsInsertWhitelist)
	t.Run("Uploads", testUploadsInsert)
	t.Run("Uploads", testUploadsInsertWhitelist)
	t.Run("Users", testUsersInsert)
	t.Run("Users", testUsersInsertWhitelist)
}

// TestToOne tests cannot be run in parallel
// or deadlocks can occur.
func TestToOne(t *testing.T) {
	t.Run("PublishQueryToUploadUsingUpload", testPublishQueryToOneUploadUsingUpload)
	t.Run("UploadToUserUsingUser", testUploadToOneUserUsingUser)
	t.Run("UserToLbrynetServerUsingLbrynetServer", testUserToOneLbrynetServerUsingLbrynetServer)
}

// TestOneToOne tests cannot be run in parallel
// or deadlocks can occur.
func TestOneToOne(t *testing.T) {
	t.Run("UploadToPublishQueryUsingPublishQuery", testUploadOneToOnePublishQueryUsingPublishQuery)
}

// TestToMany tests cannot be run in parallel
// or deadlocks can occur.
func TestToMany(t *testing.T) {
	t.Run("LbrynetServerToUsers", testLbrynetServerToManyUsers)
	t.Run("UserToUploads", testUserToManyUploads)
}

// TestToOneSet tests cannot be run in parallel
// or deadlocks can occur.
func TestToOneSet(t *testing.T) {
	t.Run("PublishQueryToUploadUsingPublishQuery", testPublishQueryToOneSetOpUploadUsingUpload)
	t.Run("UploadToUserUsingUploads", testUploadToOneSetOpUserUsingUser)
	t.Run("UserToLbrynetServerUsingUsers", testUserToOneSetOpLbrynetServerUsingLbrynetServer)
}

// TestToOneRemove tests cannot be run in parallel
// or deadlocks can occur.
func TestToOneRemove(t *testing.T) {
	t.Run("UploadToUserUsingUploads", testUploadToOneRemoveOpUserUsingUser)
	t.Run("UserToLbrynetServerUsingUsers", testUserToOneRemoveOpLbrynetServerUsingLbrynetServer)
}

// TestOneToOneSet tests cannot be run in parallel
// or deadlocks can occur.
func TestOneToOneSet(t *testing.T) {
	t.Run("UploadToPublishQueryUsingPublishQuery", testUploadOneToOneSetOpPublishQueryUsingPublishQuery)
}

// TestOneToOneRemove tests cannot be run in parallel
// or deadlocks can occur.
func TestOneToOneRemove(t *testing.T) {}

// TestToManyAdd tests cannot be run in parallel
// or deadlocks can occur.
func TestToManyAdd(t *testing.T) {
	t.Run("LbrynetServerToUsers", testLbrynetServerToManyAddOpUsers)
	t.Run("UserToUploads", testUserToManyAddOpUploads)
}

// TestToManySet tests cannot be run in parallel
// or deadlocks can occur.
func TestToManySet(t *testing.T) {
	t.Run("LbrynetServerToUsers", testLbrynetServerToManySetOpUsers)
	t.Run("UserToUploads", testUserToManySetOpUploads)
}

// TestToManyRemove tests cannot be run in parallel
// or deadlocks can occur.
func TestToManyRemove(t *testing.T) {
	t.Run("LbrynetServerToUsers", testLbrynetServerToManyRemoveOpUsers)
	t.Run("UserToUploads", testUserToManyRemoveOpUploads)
}

func TestReload(t *testing.T) {
	t.Run("GorpMigrations", testGorpMigrationsReload)
	t.Run("LbrynetServers", testLbrynetServersReload)
	t.Run("PublishQueries", testPublishQueriesReload)
	t.Run("QueryLogs", testQueryLogsReload)
	t.Run("Uploads", testUploadsReload)
	t.Run("Users", testUsersReload)
}

func TestReloadAll(t *testing.T) {
	t.Run("GorpMigrations", testGorpMigrationsReloadAll)
	t.Run("LbrynetServers", testLbrynetServersReloadAll)
	t.Run("PublishQueries", testPublishQueriesReloadAll)
	t.Run("QueryLogs", testQueryLogsReloadAll)
	t.Run("Uploads", testUploadsReloadAll)
	t.Run("Users", testUsersReloadAll)
}

func TestSelect(t *testing.T) {
	t.Run("GorpMigrations", testGorpMigrationsSelect)
	t.Run("LbrynetServers", testLbrynetServersSelect)
	t.Run("PublishQueries", testPublishQueriesSelect)
	t.Run("QueryLogs", testQueryLogsSelect)
	t.Run("Uploads", testUploadsSelect)
	t.Run("Users", testUsersSelect)
}

func TestUpdate(t *testing.T) {
	t.Run("GorpMigrations", testGorpMigrationsUpdate)
	t.Run("LbrynetServers", testLbrynetServersUpdate)
	t.Run("PublishQueries", testPublishQueriesUpdate)
	t.Run("QueryLogs", testQueryLogsUpdate)
	t.Run("Uploads", testUploadsUpdate)
	t.Run("Users", testUsersUpdate)
}

func TestSliceUpdateAll(t *testing.T) {
	t.Run("GorpMigrations", testGorpMigrationsSliceUpdateAll)
	t.Run("LbrynetServers", testLbrynetServersSliceUpdateAll)
	t.Run("PublishQueries", testPublishQueriesSliceUpdateAll)
	t.Run("QueryLogs", testQueryLogsSliceUpdateAll)
	t.Run("Uploads", testUploadsSliceUpdateAll)
	t.Run("Users", testUsersSliceUpdateAll)
}
