package migrator

type CLI struct {
	MigrateUp struct {
	} `cmd:"" help:"Apply database migrations"`
	MigrateDown struct {
		Max int `optional:"" help:"Max number of migrations to unapply" default:"0"`
	} `cmd:"" help:"Unapply database migrations"`
}
