package model

import (
	"fmt"
	"reflect"
	"time"

	"github.com/yaoapp/xun"
	"github.com/yaoapp/xun/dbal/query"
	"github.com/yaoapp/xun/dbal/schema"
	"github.com/yaoapp/xun/utils"
)

// Make make a new xun model instance
func Make(buidler *query.Builder, schema schema.Schema, v interface{}, args ...interface{}) *Model {
	if reflect.TypeOf(v).Kind() == reflect.Ptr {
		makeByStruct(buidler, schema, v)
		return nil
	}
	return makeBySchema(buidler, schema, v, args...)
}

// MakeUsing create model using makeer
func MakeUsing(maker MakerFunc, v interface{}, args ...interface{}) *Model {
	return maker(v, args...)
}

// New create a new xun model instance
func (model *Model) New(v interface{}, args ...interface{}) *Model {
	return Make(model.Builder, model.schema, v, args...)
}

// GetFullname get the fullname of model
func (model *Model) GetFullname() string {
	if model.namespace == "" {
		return model.name
	}
	return fmt.Sprintf("%s.%s", model.namespace, model.name)
}

// IsEmpty determine if the model is null
func (model *Model) IsEmpty() bool {
	return model.values.IsEmpty()
}

// GetSchema get the query interface
func (model *Model) GetSchema() schema.Schema {
	return model.schema
}

// GetName get the name of model
func (model *Model) GetName() string {
	return model.name
}

// GetNamespace get the name of model
func (model *Model) GetNamespace() string {
	return model.namespace
}

// GetAttributes get all of attribute
func (model *Model) GetAttributes() []Attribute {
	attrs := []Attribute{}
	for _, attr := range model.attributes {
		attrs = append(attrs, attr)
	}
	return attrs
}

// CleanValues clean values of Attributes
func (model *Model) CleanValues() *Model {
	model.values = xun.MakeRow()
	return model
}

// GetValues get values
func (model *Model) GetValues(with ...bool) xun.R {
	return model.values
}

// GetAttributeNames get all of the attribute name
func (model *Model) GetAttributeNames() []string {
	names := []string{}
	for name := range model.attributes {
		names = append(names, name)
	}
	return names
}

// GetAttr get the Attribute by name
func (model *Model) GetAttr(name string) *Attribute {
	attr, ok := model.attributes[name]
	if !ok {
		return nil
	}
	return &attr
}

// Clean clean the Attribute by name
func (model *Model) Clean(name string) *Model {
	attr := model.GetAttr(name)
	if attr != nil {
		model.values.Del(attr.Name)
	}
	return model
}

// Has datermind if the model has the value
func (model *Model) Has(name string) bool {
	return model.values.Has(name)
}

// Value get the Attribute value
func (model *Model) Value(name string) interface{} {
	attr := model.GetAttr(name)
	if attr == nil {
		return nil
	}
	return model.values.Get(name)
}

// Set set the Attribute value
func (model *Model) Set(name string, value interface{}, v ...interface{}) *Model {
	attr := model.GetAttr(name)
	if attr == nil {
		return model
	}
	model.values[attr.Name] = value
	if attr.Column.Field != "" && len(v) > 0 {
		setFieldValue(v[0], attr.Column.Field, value)
	}
	return model
}

// SetBind set the Attribute value
func (model *Model) SetBind(v interface{}, name string, value interface{}, fieldNames map[string]string) *Model {
	attr := model.GetAttr(name)
	if attr == nil {
		return model
	}
	model.values[attr.Name] = value
	if field, has := fieldNames[attr.Name]; has {
		setFieldValue(v, field, value)
	}
	return model
}

// Columns get the columns of model struct
func (model *Model) Columns() []*Column {
	return model.columns
}

// Searchable get the the searchable columns
func (model *Model) Searchable() []string {
	return model.searchable
}

// PrimaryKeys get the primary key columns
func (model *Model) PrimaryKeys() []string {
	return model.primaryKeys
}

// Primary get the fisrt primary key columns
func (model *Model) Primary() string {
	return model.primary
}

// Fill to fill attributes into model
func (model *Model) Fill(attributes interface{}, v ...interface{}) *Model {
	if len(v) > 0 {
		return model.FillBind(v[0], attributes)
	}
	row := xun.MakeRow(attributes)
	for name, value := range row {
		model.Set(name, value)
	}
	return model
}

// FillBind to fill attributes into model and the give var
func (model *Model) FillBind(v interface{}, attributes interface{}) *Model {
	row := xun.MakeRow(attributes)
	fieldNames := getFieldMaps(v)
	for name, value := range row {
		model.SetBind(v, name, value, fieldNames)
	}
	return model
}

// Save to create or update one model
func (model *Model) Save(v ...interface{}) error {

	if model.table.Name == "" {
		return fmt.Errorf("table name is nil, binding table first")
	}

	if model.Timestamps {
		model.Set("updated_at", time.Now().Format("2006-01-02 15:04:05.000000"))
	}

	row := model.GetValues()
	qb := model.Builder.Table(model.table.Name)

	var err error
	if row.Has(model.primary) {
		where := xun.MakeR()
		where[model.primary] = row.Get(model.primary)
		_, err = qb.UpdateOrInsert(where, row)
	} else if len(model.uniqueKeys) > 0 {
		_, err = qb.Upsert(row, model.uniqueKeys, row)
	} else {
		err = qb.Insert(row)
	}

	if len(v) > 0 {
		model.FillBind(v[0], row)
	}
	return err
}

// Get over load Get
func (model *Model) Get(v ...interface{}) ([]xun.R, error) {
	model.querySetting()
	row, err := model.Builder.Get(v...)
	return row, err
}

// MustGet over load MustGet
func (model *Model) MustGet(v ...interface{}) []xun.R {
	res, err := model.Get(v...)
	utils.PanicIF(err)
	return res
}

// First Execute the query and get the first result.
func (model *Model) First(v ...interface{}) (xun.R, error) {
	model.querySetting()
	row, err := model.Builder.First(v...)
	return row, err
}

// MustFirst Execute the query and get the first result.
func (model *Model) MustFirst(v ...interface{}) xun.R {
	res, err := model.First(v...)
	utils.PanicIF(err)
	return res
}

// MustFind find by primary key
func (model *Model) MustFind(id interface{}, v ...interface{}) *Model {
	_, err := model.Find(id, v...)
	utils.PanicIF(err)
	return model
}

// Find find by primary key
func (model *Model) Find(id interface{}, v ...interface{}) (xun.R, error) {

	if model.Invalid() != nil {
		return nil, model.Invalid()
	}

	qb := model.Builder.Table(model.table.Name)
	args := []interface{}{}
	args = append(args, model.primary)
	if len(v) == 1 {
		columns := model.explodeColumns(v[0])
		qb.Select(columns)
	}

	if model.softDeletes && model.onlyDeletes {
		qb.WhereNotNull("deleted_at")
	} else if model.softDeletes && !model.withDeletes {
		qb.WhereNull("deleted_at")
	}

	row, err := qb.Find(id, args...)
	model.resetTrashed()

	if err != nil {
		return nil, err
	}

	// fill data
	model.
		CleanValues().
		Fill(row, v...)

	return row, err
}

// Destroy deleting an dxisting model by its Primary Key
func (model *Model) Destroy(args ...interface{}) error {

	if model.Invalid() != nil {
		return model.Invalid()
	}

	ids := prepareDestroyArgs(args...)
	if len(ids) == 0 && !model.Has(model.primary) {
		return fmt.Errorf("the primary key does not set")
	}

	if len(ids) == 0 {
		ids = append(ids, model.Value(model.primary))
	}

	qb := model.Builder.Table(model.table.Name).WhereIn(model.primary, ids)
	if model.softDeletes {
		_, err := qb.Update(xun.R{"deleted_at": time.Now().Format("2006-01-02 15:04:05.000000")})
		return err
	}
	_, err := qb.Delete()

	return err
}

// Where Add a basic where clause to the query.
func (model *Model) Where(column interface{}, args ...interface{}) *Model {
	model.Builder.Where(column, args...)
	return model
}

// Select same as the Query Select
func (model *Model) Select(columns ...interface{}) *Model {
	model.Builder.Select(columns...)
	return model
}

// WithTrashed Including Soft Deleted Models
func (model *Model) WithTrashed() *Model {
	model.withDeletes = true
	return model
}

// OnlyTrashed Retrieving Only Soft Deleted Models
func (model *Model) OnlyTrashed() *Model {
	model.onlyDeletes = true
	return model
}

// With where the array key is a relationship name and the array value is a closure that adds additional constraints to the eager loading query
func (model *Model) With(args ...interface{}) *Model {
	name, closure := prepareWithArgs(args...)
	var rel *Relationship = nil
	if attr, has := model.attributes[name]; has {
		rel = attr.Relationship
	}

	if rel == nil {
		invalidArguments()
	}

	if rel.Type == "hasOne" {
		model.withHasOne(rel, name, closure)
	}

	return model
}

// withHasOne
func (model *Model) withHasOne(rel *Relationship, name string, closure func(query.Query)) {

	// if len(rel.Models) < 1 || rel.Type != "hasOne" {
	// 	invalidRelationship()
	// }

	// relModelName := rel.Models[0]
	// relFullname := relModelName
	// if !strings.Contains(relFullname, ".") {
	// 	relFullname = fmt.Sprintf("%s.%s", model.namespace, relFullname)
	// }

	// relModel := model.New(relFullname)
	// if closure != nil {
	// 	closure(relModel)
	// } else if rel.Columns != nil {
	// 	qb.Select(rel.Columns)
	// }

	// // Get Cloums
	// columns := qb.Builder.Query
	// for i := range columns {
	// 	if column, ok := columns[i].(string); ok && !strings.Contains(column, ".") {
	// 		columns[i] = fmt.Sprintf("%s.%s", name, column)
	// 	}
	// }

	// // bind local
	// local := fmt.Sprintf("%s.%s_id", model.name, strings.ToLower(relModelName))
	// foreign := fmt.Sprintf("%s.id", name)
	// if len(rel.Links) == 2 {
	// 	local = fmt.Sprintf("%s.%s", model.name, rel.Links[0])
	// 	foreign = fmt.Sprintf("%s.%s", name, rel.Links[1])
	// }

	// table := fmt.Sprintf("%s as %s", model.table.Name, model.table.Name)
	// tableWith := fmt.Sprintf("%s as %s", relModel.table.Name, name)
	// model.
	// 	Table(table).
	// 	LeftJoin(tableWith, local, "=", foreign)

	// fmt.Println("setup withHasOne: ", name, " SQL:", model.ToSQL(), " Link:", local, "<->", foreign)

}

// GetQuery return the query builder
func (model *Model) GetQuery() query.Query {

	qb := model.Builder.New()

	if model.table.Name != "" {
		qb.Table(model.table.Name)
	}

	if model.softDeletes && model.onlyDeletes {
		qb.WhereNotNull("deleted_at")
	} else if model.softDeletes && !model.withDeletes {
		qb.WhereNull("deleted_at")
	}

	return qb
}

// Reset reset query
func (model *Model) Reset() *Model {
	model.Builder.Reset()
	return model
}

func (model *Model) querySetting() {

	if model.table.Name != "" && model.Builder.Query.From.IsEmpty() {
		model.From(model.table.Name)
	}

	if model.softDeletes && model.onlyDeletes {
		model.WhereNotNull("deleted_at")
	} else if model.softDeletes && !model.withDeletes {
		model.WhereNull("deleted_at")
	}
}

// Invalid determine if the model is invalid
func (model *Model) Invalid() error {
	if model.primary == "" {
		return fmt.Errorf("The primary key does not set")
	}

	if model.table.Name == "" {
		return fmt.Errorf("The table does not set")
	}

	return nil
}

// // Create to create one model
// func (model *Model) Create(attributes interface{}) {
// }

// // Restore To restore a soft deleted model,
// func (model *Model) Restore() {
// }

// // Insert same as the query insert
// func (model *Model) Insert(v interface{}, columns ...interface{}) {
// }

// // Update  same as the query update
// func (model *Model) Update() {
// }

// // Upsert same as the query upsert
// func (model *Model) Upsert() {
// }

// // UpdateOrInsert same as the query UpdateOrInsert
// func (model *Model) UpdateOrInsert() {
// }

// // Delete same as the query Delete
// func (model *Model) Delete() {
// }

// // Truncate same as the query Truncate
// func (model *Model) Truncate() {
// }

// Search search by given params
func (model *Model) Search() interface{} {
	return nil
}

// Export export data
func (model *Model) Export() {
}

// Import import data
func (model *Model) Import() {
}

// Flow process a flow by the given flow name and return the result
func (model *Model) Flow(name string) interface{} {
	return nil
}

// FlowRaw process a flow by the given json description file and return the result
func (model *Model) FlowRaw(flow []byte) interface{} {
	return nil
}

func (model *Model) explodeColumns(v interface{}) []string {
	tags := getFieldTags(v)
	columns := model.fliterColumns(tags)
	return columns
}

func (model *Model) fliterColumns(input []string) []string {
	result := []string{}
	for _, v := range input {
		if utils.StringHave(model.columnNames, v) {
			result = append(result, v)
		}
	}
	return result
}

// resetTrashed
func (model *Model) resetTrashed() *Model {
	model.withDeletes = false
	model.onlyDeletes = false
	return model
}
