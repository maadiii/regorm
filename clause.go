package regorm

import (
	"fmt"
	"regexp"
	"strings"
)

type Builer struct {
	key        string
	value      any
	operator   string
	nextBoolOP string
}

type Clause struct {
	builder []Builer
	not     bool
}

func (w *Clause) EQ(field string, value any) *Clause {
	if w.not {
		field = NOTOperator + field
	}

	w.builder = append(w.builder, EQ(field, value).builder...)
	w.not = false

	return w
}

func (w *Clause) GT(field string, value any) *Clause {
	if w.not {
		field = NOTOperator + field
	}

	w.builder = append(w.builder, GT(field, value).builder...)
	w.not = false

	return w
}

func (w *Clause) GTE(field string, value any) *Clause {
	if w.not {
		field = NOTOperator + field
	}

	w.builder = append(w.builder, GTE(field, value).builder...)
	w.not = false

	return w
}

func (w *Clause) LT(field string, value any) *Clause {
	if w.not {
		field = NOTOperator + field
	}

	w.builder = append(w.builder, LT(field, value).builder...)
	w.not = false

	return w
}

func (w *Clause) LTE(field string, value any) *Clause {
	if w.not {
		field = NOTOperator + field
	}

	w.builder = append(w.builder, LTE(field, value).builder...)
	w.not = false

	return w
}

// NOTE: Because of golang limition in array of any([]any)
// IN function used any type for values parameter instead of []any,
// developers must call IN function with array of any.
func (w *Clause) IN(field string, values any) *Clause {
	if w.not {
		field = NOTOperator + field
	}

	w.builder = append(w.builder, IN(field, values).builder...)
	w.not = false

	return w
}

func (w *Clause) Like(field, value string) *Clause {
	if w.not {
		field = NOTOperator + field
	}

	w.builder = append(w.builder, Like(field, value).builder...)
	w.not = false

	return w
}

func (w *Clause) Between(field string, value any) *Clause {
	if w.not {
		field = NOTOperator + field
	}

	w.builder = append(w.builder, Between(field, value).builder...)
	w.not = false

	return w
}

func (w *Clause) AND() *Clause {
	w.builder[len(w.builder)-1].nextBoolOP = ANDOperator

	return w
}

func (w *Clause) OR() *Clause {
	w.builder[len(w.builder)-1].nextBoolOP = OROperator

	return w
}

func (w *Clause) NOT() *Clause {
	w.not = true

	return w
}

func (w *Clause) Search(fields []string, value, operator string) *Clause {
	w.builder = append(w.builder, Search(fields, value, operator).builder...)

	return w
}

func (w *Clause) ToSQL() []any {
	args := make([]any, 1)

	var where string

	for _, clause := range w.builder {
		if len(clause.operator) > 0 {
			where += fmt.Sprintf("%s %s ?", clause.key, clause.operator)
		} else {
			where += fmt.Sprintf("%s %s", clause.key, clause.operator)
		}

		if len(clause.nextBoolOP) > 0 {
			where += clause.nextBoolOP
		}

		if clause.value != nil {
			args = append(args, clause.value) //nolint
		}
	}

	args[0] = where

	return args
}

func EQ(field string, value any) *Clause {
	return makeWhereClause(EQOperator, field, value)
}

func GTE(field string, value any) *Clause {
	return makeWhereClause(GTEOperator, field, value)
}

func GT(field string, value any) *Clause {
	return makeWhereClause(GTOperator, field, value)
}

func LTE(field string, value any) *Clause {
	return makeWhereClause(LTEOperator, field, value)
}

func LT(field string, value any) *Clause {
	return makeWhereClause(LTOperator, field, value)
}

func Between(field string, value any) *Clause {
	return makeWhereClause(BetWeen, field, value)
}

func Like(field, value string) *Clause {
	return makeWhereClause(LikeOperator, field, value)
}

// NOTE: Because of golang limition in array of any([]any)
// IN function used any type for values parameter instead of []any,
// developers must call IN function with array of any.
func IN(field string, values any) *Clause {
	return makeWhereClause(INOperator, field, values)
}

func NOT() *Clause {
	return &Clause{
		not: true,
	}
}

func Search(fields []string, value, operator string) *Clause {
	return makeWhereClause("", generateTextSearch(fields, value, operator), nil)
}

func makeWhereClause(operator, field string, value any) *Clause {
	return &Clause{
		builder: []Builer{
			{
				key:      field,
				value:    value,
				operator: operator,
			},
		},
	}
}

func generateTextSearch(columns []string, input, operator string) (res string) {
	input = removeMultipleSpace(input)
	input = strings.TrimSpace(input)

	words := strings.Split(input, " ")
	for i := 0; i < len(words); i++ {
		words[i] += ":*"
	}

	res = "to_tsvector("
	res += strings.Join(columns, " ||' '|| ")
	res += ") @@ to_tsquery('"
	res += strings.Join(words, fmt.Sprintf(" %s ", operator))
	res += "')"

	return
}

func removeMultipleSpace(input string) string {
	space := regexp.MustCompile(`\s+`)

	return space.ReplaceAllString(input, " ")
}

const (
	EQOperator   = "="
	GTOperator   = ">"
	GTEOperator  = ">="
	LTOperator   = "<"
	LTEOperator  = "<="
	INOperator   = "IN"
	LikeOperator = "LIKE"
	BetWeen      = "BETWEEN"
	NOTOperator  = "NOT "
	OROperator   = "OR "
	ANDOperator  = "AND "
	ASCOperator  = " ASC"
	DESCOperator = " DESC"
)
