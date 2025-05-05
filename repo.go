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

type Repository[E entity] interface {
	QueryMaker[E]
	QueryConsumer[E]
	RawExecutor[E]
	ConflictResovler[E]

	SetTx(tx Transaction, commit bool) Repository[E]
}

type QueryMaker[E entity] interface {
	Where(whereClause *Clause) Repository[E]
	Having(whereClause *Clause) Repository[E]
	Select(cols ...string) Repository[E]
	Offset(value int) Repository[E]
	Limit(value int) Repository[E]
	OrderBy(name string, desc bool) Repository[E]
	GroupBy(name string) Repository[E]
	Args() []any
	IsMany() Repository[E]
	Join(arg any) Repository[E]
}

type QueryConsumer[E entity] interface { //nolint
	Find(ctx context.Context) ([]E, error)
	One(ctx context.Context) (E, error)
	Count(ctx context.Context) (int64, error)
	Exists(ctx context.Context) (bool, error)

	Insert(ctx context.Context, entity E) error
	Save(ctx context.Context, entity E) (rowsAffected int64, err error)
	InsertBatch(ctx context.Context, entities []E) (rowsAffected int64, err error)
	Update(ctx context.Context, entity E) (rowsAffected int64, err error)
	Delete(ctx context.Context) (rowsAffected int64, err error)

	InsertTx(ctx context.Context, entity E) (tx Transaction, rowsAffected int64, err error)
	SaveTx(ctx context.Context, entity E) (tx Transaction, rowsAffected int64, err error)
	InsertBatchTx(ctx context.Context, entities []E) (tx Transaction, rowsAffected int64, err error)
	UpdateTx(ctx context.Context, entity E) (tx Transaction, rowsAffected int64, err error)
	// rowsAffected param can be nil
	DeleteTx(ctx context.Context) (tx Transaction, rowsAffected int64, err error)
}

type ConflictResovler[E entity] interface {
	OnConflict(clause clause.OnConflict) Repository[E]
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

type repository[E entity] struct {
	transaction *transaction
	conflict    *clause.OnConflict
	error       error
	entity      E
	clause      *Clause
	hasMany     bool
}

func NewRepository[E entity](ent E) Repository[E] {
	return &repository[E]{
		entity:      ent,
		transaction: &transaction{scopes: make([]func(*gorm.DB) *gorm.DB, 0)},
		clause:      &Clause{builder: make([]Builer, 0)},
	}
}

func (e *repository[E]) OnConflict(c clause.OnConflict) Repository[E] {
	e.conflict = &c

	return e
}

func (e *repository[E]) Select(cols ...string) Repository[E] {
	e.transaction.scopes = append(
		e.transaction.scopes,
		func(db *gorm.DB) *gorm.DB {
			return db.Select(cols)
		},
	)

	return e
}

func (e *repository[E]) Where(whereClause *Clause) Repository[E] {
	e.clause = whereClause
	e.transaction.scopes = append(
		e.transaction.scopes,
		func(db *gorm.DB) *gorm.DB {
			args := whereClause.tosql()
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

func (e *repository[E]) OrderBy(name string, ascending bool) Repository[E] {
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

func (e *repository[E]) Offset(value int) Repository[E] {
	e.transaction.scopes = append(
		e.transaction.scopes,
		func(db *gorm.DB) *gorm.DB {
			return db.Offset(value)
		},
	)

	return e
}

func (e *repository[E]) Limit(value int) Repository[E] {
	e.transaction.scopes = append(
		e.transaction.scopes,
		func(db *gorm.DB) *gorm.DB {
			return db.Limit(value)
		},
	)

	return e
}

func (e *repository[E]) GroupBy(name string) Repository[E] {
	e.transaction.scopes = append(
		e.transaction.scopes,
		func(db *gorm.DB) *gorm.DB {
			return db.Group(name)
		},
	)

	return e
}

func (e *repository[E]) Having(whereClause *Clause) Repository[E] {
	e.clause = whereClause
	e.transaction.scopes = append(
		e.transaction.scopes,
		func(db *gorm.DB) *gorm.DB {
			return db.Having(e.clause.tosql())
		},
	)

	return e
}

func (e *repository[E]) IsMany() Repository[E] {
	e.hasMany = true

	return e
}

func (e *repository[E]) Args() []any {
	var table string

	if e.hasMany {
		title := cases.Title(language.English, cases.NoLower)
		table = title.String(e.entity.TableName())
	} else {
		table = reflect.ValueOf(e.entity).Elem().Type().Name()
	}

	args := []any{table}

	if len(e.clause.tosql()) > 1 {
		args = append(args, e.clause.tosql()...)
	}

	return args
}

func (e *repository[E]) Join(arg any) Repository[E] {
	var args []any

	switch t := arg.(type) {
	case *Clause:
		args = e.Args()
	case entity:
		args = NewRepository(t).Args()
	default:
		panic("arg param just accept *Clause or entity type")
	}

	// if _, ok := arg.(*Clause); ok {
	// 	args = e.Args()
	// } else {
	// 	v := newVar(arg).(entity)
	// 	args = NewRepository(v).Args()
	// }

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

func (e *repository[E]) Find(ctx context.Context) ([]E, error) {
	result := make([]E, 0)

	err := db.WithContext(ctx).Scopes(e.transaction.scopes...).Find(&result).Error
	if err != nil {
		return nil, e.joinError(err)
	}

	return result, err
}

func (e *repository[E]) One(ctx context.Context) (E, error) {
	var result E

	err := db.WithContext(ctx).Scopes(e.transaction.scopes...).First(&result).Error
	if err != nil {
		return result, e.joinError(err)
	}

	return result, nil
}

func (e *repository[E]) Count(ctx context.Context) (int64, error) {
	var count int64

	err := db.WithContext(ctx).
		Model(e.entity).
		Scopes(e.transaction.scopes...).
		Count(&count).Error
	if err != nil {
		return -1, e.joinError(err)
	}

	return count, nil
}

func (e *repository[E]) Exists(ctx context.Context) (bool, error) {
	var count int64

	err := db.WithContext(ctx).
		Model(e.entity).
		Scopes(e.transaction.scopes...).
		Count(&count).Error
	if err != nil {
		return false, e.joinError(err)
	}

	return count > 0, nil
}

func (e *repository[E]) Insert(ctx context.Context, entity E) error {
	if e.transaction.tx == nil {
		res := db.WithContext(ctx)
		if e.conflict != nil {
			res = res.Clauses(e.conflict)
		}

		res = res.Create(entity)

		return res.Error
	}

	res := db.WithContext(ctx)
	if e.conflict != nil {
		res = res.Clauses(e.conflict)
	}

	res = res.Create(entity)
	if res.Error != nil {
		if _, err := e.rollback(res.Error); err != nil {
			return e.joinError(err)
		}

		return e.joinError(res.Error)
	}

	if _, err := e.commit(); err != nil {
		return e.joinError(err)
	}

	return nil
}

func (e *repository[E]) Save(ctx context.Context, entity E) (rowsAffected int64, err error) {
	if e.transaction.tx == nil {
		res := db.WithContext(ctx)
		if e.conflict != nil {
			res = res.Clauses(e.conflict)
		}

		res = res.Save(entity)

		return res.RowsAffected, res.Error
	}

	res := e.transaction.tx.WithContext(ctx)
	if e.conflict != nil {
		res = res.Clauses(e.conflict)
	}

	res = res.Save(entity)
	if res.Error != nil {
		if _, err = e.rollback(res.Error); err != nil {
			return 0, e.joinError(err)
		}

		return 0, e.joinError(res.Error)
	}

	if _, err := e.commit(); err != nil {
		return 0, e.joinError(err)
	}

	return res.RowsAffected, nil
}

func (e *repository[E]) InsertBatch(ctx context.Context, entities []E) (rowsAffected int64, err error) {
	if e.transaction.tx == nil {
		res := db.WithContext(ctx)
		if e.conflict != nil {
			res = res.Clauses(e.conflict)
		}

		res = res.CreateInBatches(entities, len(entities))

		return res.RowsAffected, res.Error
	}

	res := e.transaction.tx.WithContext(ctx)
	if e.conflict != nil {
		res = res.Clauses(e.conflict)
	}

	res = res.CreateInBatches(entities, len(entities))
	if res.Error != nil {
		if _, err = e.rollback(res.Error); err != nil {
			return 0, e.joinError(err)
		}

		return 0, e.joinError(res.Error)
	}

	if _, err = e.commit(); err != nil {
		return 0, e.joinError(err)
	}

	return res.RowsAffected, nil
}

func (e *repository[E]) InsertTx(ctx context.Context, entity E) (tx Transaction, rowsAffected int64, err error) {
	e.transaction.tx = db.WithContext(ctx).Begin()

	res := e.transaction.tx
	if e.conflict != nil {
		res = res.Clauses(e.conflict)
	}

	res = res.Create(entity)
	if res.Error != nil {
		tx, err = e.rollback(res.Error)

		return tx, 0, err
	}

	tx, err = e.commit()
	if err != nil {
		return nil, 0, errors.Join(err)
	}

	return tx, res.RowsAffected, err
}

func (e *repository[E]) SaveTx(ctx context.Context, entity E) (tx Transaction, rowsAffected int64, err error) {
	e.transaction.tx = db.WithContext(ctx).Begin()

	res := e.transaction.tx
	if e.conflict != nil {
		res = res.Clauses(e.conflict)
	}

	res = res.Save(entity)
	if res.Error != nil {
		tx, err = e.rollback(res.Error)

		return tx, 0, err
	}

	tx, err = e.commit()
	if err != nil {
		return nil, 0, errors.Join(err)
	}

	return tx, res.RowsAffected, err
}

func (e *repository[E]) InsertBatchTx(ctx context.Context, entities []E) (tx Transaction, rowsAffected int64, err error) {
	e.transaction.tx = db.WithContext(ctx).Begin()

	res := e.transaction.tx
	if e.conflict != nil {
		res = res.Clauses(e.conflict)
	}

	res = res.CreateInBatches(entities, len(entities))
	if res.Error != nil {
		tx, err = e.rollback(res.Error)

		return tx, 0, err
	}

	tx, err = e.commit()
	if err != nil {
		return nil, 0, errors.Join(err)
	}

	return tx, res.RowsAffected, err
}

func (e *repository[E]) Update(ctx context.Context, entity E) (rowsAffected int64, err error) {
	if e.transaction.tx == nil {
		res := db.WithContext(ctx)
		if e.conflict != nil {
			res = res.Clauses(e.conflict)
		}

		res.Scopes(e.transaction.scopes...).Updates(entity)

		return res.RowsAffected, res.Error
	}

	res := e.transaction.tx.WithContext(ctx).
		Clauses(e.conflict).
		Scopes(e.transaction.scopes...).
		Updates(entity)
	if res.Error != nil {
		if _, err := e.rollback(res.Error); err != nil {
			return 0, e.joinError(err)
		}

		return 0, e.joinError(res.Error)
	}

	if _, err := e.commit(); err != nil {
		return 0, e.joinError(err)
	}

	return res.RowsAffected, nil
}

func (e *repository[E]) UpdateTx(ctx context.Context, entity E) (tx Transaction, rowsAffected int64, err error) {
	e.transaction.tx = db.Begin()

	res := e.transaction.tx.WithContext(ctx).
		Clauses(e.conflict).
		Scopes(e.transaction.scopes...).
		Updates(entity)
	if res.Error != nil {
		tx, err = e.rollback(res.Error)

		return tx, 0, err
	}

	tx, err = e.commit()
	if err != nil {
		return nil, 0, errors.Join(err)
	}

	return tx, res.RowsAffected, err
}

func (e *repository[E]) Delete(ctx context.Context) (rowsAffected int64, err error) {
	if e.transaction.tx == nil {
		res := db.WithContext(ctx).
			Scopes(e.transaction.scopes...).
			Delete(e.entity)

		return res.RowsAffected, res.Error
	}

	res := e.transaction.tx.WithContext(ctx).
		Scopes(e.transaction.scopes...).
		Delete(e.entity)
	if res.Error != nil {
		if _, err := e.rollback(res.Error); err != nil {
			return 0, e.joinError(err)
		}

		return 0, e.joinError(res.Error)
	}

	if _, err := e.commit(); err != nil {
		return 0, e.joinError(err)
	}

	return res.RowsAffected, nil
}

func (e *repository[E]) DeleteTx(ctx context.Context) (tx Transaction, rowsAffected int64, err error) {
	e.transaction.tx = db.Begin()

	res := e.transaction.tx.WithContext(ctx).
		Scopes(e.transaction.scopes...).
		Delete(e.entity)
	if res.Error != nil {
		tx, err = e.rollback(res.Error)

		return tx, 0, err
	}

	tx, err = e.commit()
	if err != nil {
		return nil, 0, errors.Join(err)
	}

	return tx, res.RowsAffected, err
}

func (e *repository[E]) SetTx(tx Transaction, commit bool) Repository[E] {
	e.transaction.tx = tx.(*transaction).tx
	e.transaction.commit = commit

	return e
}

func (e *repository[E]) Query(sql string, values ...any) error {
	err := db.Scopes(e.transaction.scopes...).
		Raw(sql, values...).
		Scan(&e.entity).Error
	if err != nil {
		return e.joinError(err)
	}

	return nil
}

func (e *repository[E]) QueryRows(sql string, values ...any) ([]E, error) {
	result := make([]E, 0)

	err := db.Scopes(e.transaction.scopes...).
		Raw(sql, values...).
		Scan(&e.entity).Error
	if err != nil {
		return nil, e.joinError(err)
	}

	return result, nil
}

func (e *repository[E]) Exec(sql string, values ...any) error {
	err := db.Scopes(e.transaction.scopes...).
		Exec(sql, values...).
		Error
	if err != nil {
		return e.joinError(err)
	}

	return nil
}

func (e *repository[E]) commit() (tx Transaction, err error) {
	if e.transaction.commit {
		return e.transaction, e.transaction.tx.Commit().Error
	}

	return e.transaction, nil
}

func (e *repository[E]) rollback(err error) (Transaction, error) {
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

func (e *repository[E]) joinError(err error) error {
	if errors.Unwrap(e.error) != nil {
		return errors.Join(e.error, err)
	}

	e.error = err

	return e.error
}

func newVar(v any) entity {
	t := reflect.TypeOf(v)

	return reflect.New(t.Elem()).Interface().(entity)
}
