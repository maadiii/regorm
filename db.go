package regorm

import (
	"database/sql"

	"gorm.io/gorm"
)

var db *gorm.DB

func Init(gormdb *gorm.DB) {
	db = gormdb
}

func Connection() (*sql.DB, error) {
	return db.DB()
}

var (
	// ErrRecordNotFound record not found error.
	ErrRecordNotFound = gorm.ErrRecordNotFound
	// ErrInvalidTransaction invalid transaction when you are trying to `Commit` or `Rollback`.
	ErrInvalidTransaction = gorm.ErrInvalidTransaction
	// ErrNotImplemented not implemented.
	ErrNotImplemented = gorm.ErrNotImplemented
	// ErrMissingWhereClause missing where clause.
	ErrMissingWhereClause = gorm.ErrMissingWhereClause
	// ErrUnsupportedRelation unsupported relations.
	ErrUnsupportedRelation = gorm.ErrUnsupportedRelation
	// ErrPrimaryKeyRequired primary keys required.
	ErrPrimaryKeyRequired = gorm.ErrPrimaryKeyRequired
	// ErrModelValueRequired model value required.
	ErrModelValueRequired = gorm.ErrModelValueRequired
	// ErrModelAccessibleFieldsRequired model accessible fields required.
	ErrModelAccessibleFieldsRequired = gorm.ErrModelAccessibleFieldsRequired
	// ErrSubQueryRequired sub query required.
	ErrSubQueryRequired = gorm.ErrSubQueryRequired
	// ErrInvalidData unsupported data.
	ErrInvalidData = gorm.ErrInvalidData
	// ErrUnsupportedDriver unsupported driver.
	ErrUnsupportedDriver = gorm.ErrUnsupportedDriver
	// ErrRegistered registered.
	ErrRegistered = gorm.ErrRegistered
	// ErrInvalidField invalid field.
	ErrInvalidField = gorm.ErrInvalidField
	// ErrEmptySlice empty slice found.
	ErrEmptySlice = gorm.ErrEmptySlice
	// ErrDryRunModeUnsupported dry run mode unsupported.
	ErrDryRunModeUnsupported = gorm.ErrDryRunModeUnsupported
	// ErrInvalidDB invalid db.
	ErrInvalidDB = gorm.ErrInvalidDB
	// ErrInvalidValue invalid value.
	ErrInvalidValue = gorm.ErrInvalidValue
	// ErrInvalidValueOfLength invalid values do not match length.
	ErrInvalidValueOfLength = gorm.ErrInvalidValueOfLength
	// ErrPreloadNotAllowed preload is not allowed when count is used.
	ErrPreloadNotAllowed = gorm.ErrPreloadNotAllowed
	// ErrDuplicatedKey occurs when there is a unique key constraint violation.
	ErrDuplicatedKey = gorm.ErrDuplicatedKey
)
