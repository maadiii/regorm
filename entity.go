package regorm

import (
	"context"
	"errors"
	"reflect"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Entitier[E entity] interface {
	SQL() Entitier[E]

	QueryMaker[E]
	QueryConsumer[E]
	RawExecutor[E]
	ConflictResovler[E]

	SetTx(tx Transaction, commit bool) Entitier[E]
}

type QueryMaker[E entity] interface {
	Where(whereClause *Clause) Entitier[E]
	Having(whereClause *Clause) Entitier[E]
	Select(cols ...string) Entitier[E]
	Offset(value int) Entitier[E]
	Limit(value int) Entitier[E]
	OrderBy(name string, desc bool) Entitier[E]
	GroupBy(name string) Entitier[E]
	ToSQL() []any
	IsMany() Entitier[E]
	Join(arg any) Entitier[E]
}

type QueryConsumer[E entity] interface { //nolint
	Find(ctx context.Context) ([]E, error)
	One(ctx context.Context) (E, error)
	Count(ctx context.Context) (int64, error)

	Insert(ctx context.Context, rowsAffected *int64) error
	Save(ctx context.Context) error
	InsertBatch(ctx context.Context, entities []E, rowsAffected *int64) error
	// rowsAffected param can be nil
	Update(ctx context.Context, rowsAffected *int64) error
	// rowsAffected param can be nil
	Delete(ctx context.Context, rowsAffected *int64) error

	InsertTx(ctx context.Context, rowsAffected *int64) (Transaction, error)
	SaveTx(ctx context.Context) (Transaction, error)
	// rowsAffected param can be nil
	UpdateTx(ctx context.Context, rowsAffected *int64) (Transaction, error)
	// rowsAffected param can be nil
	DeleteTx(ctx context.Context, rowsAffected *int64) (Transaction, error)
}

type ConflictResovler[E entity] interface {
	OnConflict(clause clause.OnConflict) Entitier[E]
}

type RawExecutor[E entity] interface {
	Query(sql string, values ...any) error
	QueryRows(sql string, values ...any) ([]E, error)
	Exec(sql string, values ...any) error
}

type entity interface {
	TableName() string
}

type Transaction interface {
	implement()
	Commit() error
	Rollback() error
}

type transaction struct {
	scopes    []func(*gorm.DB) *gorm.DB
	tx        *gorm.DB
	commit    bool
	savePoint string
}

func (t *transaction) implement() {}

func (t *transaction) Commit() error {
	return t.tx.Commit().Error
}

func (t *transaction) Rollback() error {
	return t.tx.Rollback().Error
}

type Entity[E entity] struct {
	transaction *transaction
	conflict    clause.OnConflict
	error       error
	table       E
	clause      *Clause
	hasMany     bool
}

func SQL[E entity](ent E) Entitier[E] {
	return &Entity[E]{
		table:       ent,
		transaction: &transaction{scopes: make([]func(*gorm.DB) *gorm.DB, 0)},
		clause:      &Clause{builder: make([]Builer, 0)},
	}
}

func (e *Entity[E]) SQL() Entitier[E] {
	// type A struct{}
	// func (a A) SQL() regorm.Entitier[E]{
	//   return regorm.SQL(a)
	// }
	panic("implement by yourself like above code")
}

func (e *Entity[E]) OnConflict(c clause.OnConflict) Entitier[E] {
	e.conflict = c

	return e
}

func (e *Entity[E]) Select(cols ...string) Entitier[E] {
	e.transaction.scopes = append(
		e.transaction.scopes,
		func(db *gorm.DB) *gorm.DB {
			return db.Select(cols)
		},
	)

	return e
}

func (e *Entity[E]) Where(whereClause *Clause) Entitier[E] {
	e.clause = whereClause
	e.transaction.scopes = append(
		e.transaction.scopes,
		func(db *gorm.DB) *gorm.DB {
			args := whereClause.ToSQL()
			if len(args) > 1 {
				return db.Where(args[0], args[1:]...)
			}

			if len(args) > 0 {
				return db.Where(args[0])
			}

			return nil
		},
	)

	return e
}

func (e *Entity[E]) OrderBy(name string, ascending bool) Entitier[E] {
	e.transaction.scopes = append(
		e.transaction.scopes,
		func(db *gorm.DB) *gorm.DB {
			if ascending {
				return db.Order(name + " ASC ")
			}

			return db.Order(name + " DESC ")
		},
	)

	return e
}

func (e *Entity[E]) Offset(value int) Entitier[E] {
	e.transaction.scopes = append(
		e.transaction.scopes,
		func(db *gorm.DB) *gorm.DB {
			return db.Offset(value)
		},
	)

	return e
}

func (e *Entity[E]) Limit(value int) Entitier[E] {
	e.transaction.scopes = append(
		e.transaction.scopes,
		func(db *gorm.DB) *gorm.DB {
			return db.Limit(value)
		},
	)

	return e
}

func (e *Entity[E]) GroupBy(name string) Entitier[E] {
	e.transaction.scopes = append(
		e.transaction.scopes,
		func(db *gorm.DB) *gorm.DB {
			return db.Group(name)
		},
	)

	return e
}

func (e *Entity[E]) Having(whereClause *Clause) Entitier[E] {
	e.clause = whereClause
	e.transaction.scopes = append(
		e.transaction.scopes,
		func(db *gorm.DB) *gorm.DB {
			return db.Having(e.clause.ToSQL())
		},
	)

	return e
}

func (e *Entity[E]) IsMany() Entitier[E] {
	e.hasMany = true

	return e
}

func (e *Entity[E]) ToSQL() []any {
	var table string

	if e.hasMany {
		title := cases.Title(language.English, cases.NoLower)
		table = title.String(e.table.TableName())
	} else {
		table = reflect.ValueOf(e.table).Elem().Type().Name()
	}

	args := []any{table}

	if len(e.clause.ToSQL()) > 1 {
		args = append(args, e.clause.ToSQL()...)
	}

	return args
}

func (e *Entity[E]) Join(arg any) Entitier[E] {
	var args []any

	if _, ok := arg.(*Clause); ok {
		args = e.ToSQL()
	} else {
		v := newVar(arg).(entity)
		args = SQL(v).ToSQL()
	}

	table := args[0].(string)

	if len(args) > 1 {
		query := args[1].(string)
		splited := strings.Split(query, " = ")

		var splitedStmt []string

		for _, s := range splited {
			words := strings.Split(s, " ")
			for _, word := range words {
				if len(word) > 0 {
					splitedStmt = append(splitedStmt, word)
				}
			}
		}

		for i := 1; i < len(splitedStmt); i++ {
			if splitedStmt[i] == "?" {
				splitedStmt[i-1] = table + "." + splitedStmt[i-1] + " ="
			}
		}

		stmt := strings.Join(splitedStmt, " ")

		e.transaction.scopes = append(
			e.transaction.scopes,
			func(db *gorm.DB) *gorm.DB {
				return db.Joins(table, db.Where(stmt, args[2:])) //nolint
			},
		)
	} else {
		e.transaction.scopes = append(
			e.transaction.scopes,
			func(db *gorm.DB) *gorm.DB {
				return db.Preload(table)
			},
		)
	}

	return e
}

func (e *Entity[E]) Find(ctx context.Context) ([]E, error) {
	result := make([]E, 0)

	err := db.WithContext(ctx).Scopes(e.transaction.scopes...).Find(&result).Error
	if err != nil {
		return nil, e.joinError(err)
	}

	return result, err
}

func (e *Entity[E]) One(ctx context.Context) (E, error) {
	var result E

	err := db.WithContext(ctx).Scopes(e.transaction.scopes...).First(&result).Error
	if err != nil {
		return result, e.joinError(err)
	}

	return result, nil
}

func (e *Entity[E]) Count(ctx context.Context) (int64, error) {
	var count int64

	err := db.WithContext(ctx).
		Model(e.table).
		Scopes(e.transaction.scopes...).
		Count(&count).Error
	if err != nil {
		return -1, e.joinError(err)
	}

	return count, nil
}

func (e *Entity[E]) Insert(ctx context.Context, rowsAffected *int64) error {
	if e.transaction.tx == nil {
		res := db.WithContext(ctx).Clauses(e.conflict).Create(e.table)

		if rowsAffected != nil {
			*rowsAffected = res.RowsAffected
		}

		return res.Error
	}

	res := e.transaction.tx.WithContext(ctx).Clauses(e.conflict).Create(e.table)
	if res.Error != nil {
		_, rerr := e.rollback(res.Error)
		if rerr != nil {
			return e.joinError(rerr)
		}

		return e.joinError(res.Error)
	}

	if e.transaction.commit {
		_, err := e.commit()
		if err != nil {
			return e.joinError(err)
		}
	}

	if rowsAffected != nil {
		*rowsAffected = res.RowsAffected
	}

	return nil
}

func (e *Entity[E]) Save(ctx context.Context) error {
	if e.transaction.tx == nil {
		return db.WithContext(ctx).
			Clauses(e.conflict).
			Save(e.table).Error
	}

	err := e.transaction.tx.WithContext(ctx).
		Clauses(e.conflict).
		Save(e.table).Error
	if err != nil {
		_, rerr := e.rollback(err)
		if rerr != nil {
			return e.joinError(rerr)
		}

		return e.joinError(err)
	}

	if e.transaction.commit {
		_, err := e.commit()
		if err != nil {
			return e.joinError(err)
		}
	}

	return nil
}

func (e *Entity[E]) InsertBatch(ctx context.Context, entities []E, rowsAffected *int64) error {
	if e.transaction.tx == nil {
		res := db.WithContext(ctx).
			Clauses(e.conflict).
			CreateInBatches(entities, len(entities))

		if rowsAffected != nil {
			*rowsAffected = res.RowsAffected
		}

		return res.Error
	}

	res := e.transaction.tx.WithContext(ctx).
		Clauses(e.conflict).
		CreateInBatches(entities, len(entities))
	if res.Error != nil {
		_, rerr := e.rollback(res.Error)
		if rerr != nil {
			return e.joinError(rerr)
		}

		return e.joinError(res.Error)
	}

	if e.transaction.commit {
		_, err := e.commit()
		if err != nil {
			return e.joinError(err)
		}
	}

	if rowsAffected != nil {
		*rowsAffected = res.RowsAffected
	}

	return nil
}

func (e *Entity[E]) InsertTx(ctx context.Context, rowsAffected *int64) (tx Transaction, err error) {
	e.transaction.tx = db.WithContext(ctx).Begin()

	res := e.transaction.tx.
		Clauses(e.conflict).
		Create(e.table)
	if res.Error != nil {
		return e.rollback(res.Error)
	}

	if rowsAffected != nil {
		*rowsAffected = res.RowsAffected
	}

	return e.commit()
}

func (e *Entity[E]) SaveTx(ctx context.Context) (tx Transaction, err error) {
	e.transaction.tx = db.WithContext(ctx).Begin()

	err = e.transaction.tx.
		Clauses(e.conflict).
		Save(e.table).Error
	if err != nil {
		return e.rollback(err)
	}

	return e.commit()
}

func (e *Entity[E]) Update(ctx context.Context, rowsAffected *int64) error {
	if e.transaction.tx == nil {
		res := db.WithContext(ctx).
			Clauses(e.conflict).
			Scopes(e.transaction.scopes...).
			Updates(e.table)

		if rowsAffected != nil {
			*rowsAffected = res.RowsAffected
		}

		return res.Error
	}

	res := e.transaction.tx.WithContext(ctx).
		Clauses(e.conflict).
		Scopes(e.transaction.scopes...).
		Updates(e.table)
	if res.Error != nil {
		_, rerr := e.rollback(res.Error)
		if rerr != nil {
			return e.joinError(rerr)
		}

		return e.joinError(res.Error)
	}

	if e.transaction.commit {
		_, err := e.commit()
		if err != nil {
			return e.joinError(err)
		}
	}

	if rowsAffected != nil {
		*rowsAffected = res.RowsAffected
	}

	return nil
}

func (e *Entity[E]) UpdateTx(ctx context.Context, rowsAffected *int64) (tx Transaction, err error) {
	e.transaction.tx = db.Begin()

	res := e.transaction.tx.WithContext(ctx).
		Clauses(e.conflict).
		Scopes(e.transaction.scopes...).
		Updates(e.table)
	if res.Error != nil {
		return e.rollback(res.Error)
	}

	if rowsAffected != nil {
		*rowsAffected = res.RowsAffected
	}

	return e.commit()
}

func (e *Entity[E]) Delete(ctx context.Context, rowsAffected *int64) error {
	if e.transaction.tx == nil {
		res := db.WithContext(ctx).
			Scopes(e.transaction.scopes...).
			Delete(e.table)

		if rowsAffected != nil {
			*rowsAffected = res.RowsAffected
		}

		return res.Error
	}

	res := e.transaction.tx.WithContext(ctx).
		Scopes(e.transaction.scopes...).
		Delete(e.table)
	if res.Error != nil {
		_, rerr := e.rollback(res.Error)
		if rerr != nil {
			return e.joinError(rerr)
		}

		return e.joinError(res.Error)
	}

	if e.transaction.commit {
		_, err := e.commit()
		if err != nil {
			return e.joinError(err)
		}
	}

	if rowsAffected != nil {
		*rowsAffected = res.RowsAffected
	}

	return nil
}

func (e *Entity[E]) DeleteTx(ctx context.Context, rowsAffected *int64) (tx Transaction, err error) {
	e.transaction.tx = db.Begin()

	res := e.transaction.tx.WithContext(ctx).
		Scopes(e.transaction.scopes...).
		Delete(e.table)
	if res.Error != nil {
		return e.rollback(res.Error)
	}

	if rowsAffected != nil {
		*rowsAffected = res.RowsAffected
	}

	return e.commit()
}

func (e *Entity[E]) SetTx(tx Transaction, commit bool) Entitier[E] {
	e.transaction.tx = tx.(*transaction).tx
	e.transaction.commit = commit

	return e
}

func (e *Entity[E]) Query(sql string, values ...any) error {
	err := db.Scopes(e.transaction.scopes...).
		Raw(sql, values...).
		Scan(&e.table).Error
	if err != nil {
		return e.joinError(err)
	}

	return nil
}

func (e *Entity[E]) QueryRows(sql string, values ...any) ([]E, error) {
	result := make([]E, 0)

	err := db.Scopes(e.transaction.scopes...).
		Raw(sql, values...).
		Scan(&e.table).Error
	if err != nil {
		return nil, e.joinError(err)
	}

	return result, nil
}

func (e *Entity[E]) Exec(sql string, values ...any) error {
	err := db.Scopes(e.transaction.scopes...).
		Exec(sql, values...).
		Error
	if err != nil {
		return e.joinError(err)
	}

	return nil
}

func (e *Entity[E]) commit() (tx Transaction, err error) {
	if e.transaction.commit {
		return e.transaction, e.transaction.tx.Commit().Error
	}

	return e.transaction, nil
}

func (e *Entity[E]) rollback(err error) (Transaction, error) {
	if len(e.transaction.savePoint) > 0 {
		rErr := e.transaction.tx.
			RollbackTo(e.transaction.savePoint).
			Error
		if rErr != nil {
			e.error = rErr
		}

		return nil, e.joinError(err)
	}

	rErr := e.transaction.tx.Rollback().Error
	if rErr != nil {
		e.error = rErr

		return nil, e.joinError(err)
	}

	return e.transaction, nil
}

func (e *Entity[E]) joinError(err error) error {
	if errors.Unwrap(e.error) != nil {
		return errors.Join(e.error, err)
	}

	e.error = err

	return e.error
}

func newVar(v any) any {
	t := reflect.TypeOf(v)

	return reflect.New(t.Elem()).Interface()
}
