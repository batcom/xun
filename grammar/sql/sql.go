package sql

import (
	"github.com/yaoapp/xun/dbal"
	"github.com/yaoapp/xun/utils"
)

// SQL the SQL Grammar
type SQL struct {
	Driver     string
	Mode       string
	Types      map[string]string
	FlipTypes  map[string]string
	IndexTypes map[string]string
	DSN        string
	DB         string
	Schema     string
	dbal.Grammar
	dbal.Quoter
}

// NewSQL create a new SQL instance
func NewSQL(quoter dbal.Quoter) SQL {
	sql := &SQL{
		Driver: "sql",
		Mode:   "production",
		Quoter: quoter,
		IndexTypes: map[string]string{
			"unique": "UNIQUE KEY",
			"index":  "KEY",
		},
		FlipTypes: map[string]string{},
		Types: map[string]string{
			"bigInteger":    "BIGINT",
			"smallInteger":  "SMALLINT",
			"integer":       "INT",
			"decimal":       "DECIMAL",
			"float":         "FLOAT",
			"double":        "DOUBLE",
			"string":        "VARCHAR",
			"char":          "CHAR",
			"text":          "TEXT",
			"mediumText":    "MEDIUMTEXT",
			"longText":      "LONGTEXT",
			"date":          "DATE",
			"dateTime":      "DATETIME",
			"dateTimeTz":    "DATETIME",
			"time":          "TIME",
			"timeTz":        "TIME",
			"binary":        "binary",
			"boolean":       "boolean",
			"enum":          "enum",
			"json":          "JSON",
			"jsonb":         "JSONB",
			"mediumInteger": "mediumInteger",
			"timestamp":     "timestamp",
			"timestampsTz":  "timestampsTz",
			"tinyInteger":   "tinyInteger",
			"uuid":          "UUID",
			"year":          "YEAR",
		},
	}
	return *sql
}

// New Create a new mysql grammar inteface
func New(dsn string) dbal.Grammar {
	sql := NewSQL(Quoter{})
	flipTypes, ok := utils.MapFilp(sql.Types)
	if ok {
		sql.FlipTypes = flipTypes.(map[string]string)
	}
	return &sql
}