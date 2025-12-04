package cmd

// Test exports - available only during testing.
// NOT part of the public API.

// Function exports
var (
	DeleteGraphWithDeleter = deleteGraphWithDeleter
	ExecuteBulkDelete      = executeBulkDelete
	ExecuteSingleDelete    = executeSingleDelete
	ExecuteStats           = executeStats
	ExecuteTagAdd          = executeTagAdd
	OutputStatsTable       = outputStatsTable
	OutputVersionsTable    = outputVersionsTable
	CalculateStats         = calculateStats
)

// Type aliases for test access
type (
	BulkDeleteParams    = bulkDeleteParams
	DeleteVersionParams = deleteVersionParams
	PackageStats        = packageStats
	StatsParams         = statsParams
	TagAddParams        = tagAddParams
)

// Interface aliases for test access
type (
	PackageDeleter = packageDeleter
	TagAdder       = tagAdder
	VersionLister  = versionLister
)
