package sql

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/blang/semver/v4"
	"github.com/jmoiron/sqlx"
	"github.com/yaoapp/xun/dbal"
	"github.com/yaoapp/xun/logger"
	"github.com/yaoapp/xun/utils"
)

// Config set the configure using DSN
func (grammarSQL *SQL) Config(dsn string) {
	grammarSQL.DSN = dsn
	uinfo, err := url.Parse(grammarSQL.DSN)
	if err != nil {
		panic(err)
	}
	grammarSQL.DB = filepath.Base(uinfo.Path)
	grammarSQL.Schema = grammarSQL.DB
}

// GetDBName get the database name of the current connection
func (grammarSQL SQL) GetDBName() string {
	return grammarSQL.DB
}

// GetSchemaName get the schema name of the current connection
func (grammarSQL SQL) GetSchemaName() string {
	return grammarSQL.Schema
}

// GetVersion get the version of the connection database
func (grammarSQL SQL) GetVersion(db *sqlx.DB) (*dbal.Version, error) {
	sql := fmt.Sprintf("SELECT VERSION()")
	defer logger.Debug(logger.RETRIEVE, sql).TimeCost(time.Now())
	rows := []string{}
	err := db.Select(&rows, sql)
	if err != nil {
		return nil, err
	}
	if len(rows) < 1 {
		return nil, fmt.Errorf("Can't get the version")
	}

	ver, err := semver.Make(rows[0])
	if err != nil {
		return nil, err
	}

	return &dbal.Version{
		Version: ver,
		Driver:  grammarSQL.Driver,
	}, nil
}

// TableExists check if the table exists
func (grammarSQL SQL) TableExists(name string, db *sqlx.DB) bool {
	sql := fmt.Sprintf("SHOW TABLES like %s", grammarSQL.Quoter.VAL(name, db))
	defer logger.Debug(logger.RETRIEVE, sql).TimeCost(time.Now())
	row := db.QueryRowx(sql)
	if row.Err() != nil {
		panic(row.Err())
	}
	res, err := row.SliceScan()
	if err != nil {
		return false
	}
	return name == fmt.Sprintf("%s", res[0])
}

// GetTable get a table on the schema
func (grammarSQL SQL) GetTable(table *dbal.Table, db *sqlx.DB) error {
	columns, err := grammarSQL.GetColumnListing(table.DBName, table.TableName, db)
	if err != nil {
		return err
	}

	indexes, err := grammarSQL.GetIndexListing(table.DBName, table.TableName, db)
	if err != nil {
		return err
	}

	primaryKeyName := ""

	// attaching columns
	for _, column := range columns {
		column.Indexes = []*dbal.Index{}
		table.PushColumn(column)
	}

	// attaching indexes
	for i := range indexes {
		idx := indexes[i]
		if !table.HasColumn(idx.ColumnName) {
			return errors.New("the column does not exists" + idx.ColumnName)
		}
		column := table.ColumnMap[idx.ColumnName]
		if !table.HasIndex(idx.Name) {
			index := *idx
			index.Columns = []*dbal.Column{}
			column.Indexes = append(column.Indexes, &index)
			table.PushIndex(&index)
		}
		index := table.IndexMap[idx.Name]
		index.AddColumn(column)
		if index.Type == "primary" {
			primaryKeyName = idx.Name
		}
	}

	// attaching primary
	if _, has := table.IndexMap[primaryKeyName]; has {
		idx := table.IndexMap[primaryKeyName]
		table.Primary = &dbal.Primary{
			Name:      idx.Name,
			TableName: idx.TableName,
			DBName:    idx.DBName,
			Table:     idx.Table,
			Columns:   idx.Columns,
		}
		delete(table.IndexMap, idx.Name)
		for _, column := range table.Primary.Columns {
			column.Primary = true
			column.Indexes = []*dbal.Index{}
		}
	}

	return nil
}

// GetIndexListing get a table indexes structure
func (grammarSQL SQL) GetIndexListing(dbName string, tableName string, db *sqlx.DB) ([]*dbal.Index, error) {
	selectColumns := []string{
		"`TABLE_SCHEMA` AS `db_name`",
		"`TABLE_NAME` AS `table_name`",
		"`INDEX_NAME` AS `index_name`",
		"`COLUMN_NAME` AS `column_name`",
		"`COLLATION` AS `collation`",
		`CASE
			WHEN NULLABLE = 'YES' THEN true
			WHEN NULLABLE = "NO" THEN false
			ELSE false
		END AS ` + "`nullable`",
		`CASE
			WHEN NON_UNIQUE = 0 THEN true
			WHEN NON_UNIQUE = 1 THEN false
			ELSE 0
		END AS ` + "`unique`",
		"`COMMENT` AS `comment`",
		"`INDEX_TYPE` AS `index_type`",
		"`SEQ_IN_INDEX` AS `seq_in_index`",
		"`INDEX_COMMENT` AS `index_comment`",
	}
	sql := fmt.Sprintf(`
			SELECT %s
			FROM INFORMATION_SCHEMA.STATISTICS
			WHERE TABLE_SCHEMA = %s AND TABLE_NAME = %s
			ORDER BY SEQ_IN_INDEX;
		`,
		strings.Join(selectColumns, ","),
		grammarSQL.Quoter.VAL(dbName, db),
		grammarSQL.Quoter.VAL(tableName, db),
	)
	defer logger.Debug(logger.RETRIEVE, sql).TimeCost(time.Now())
	indexes := []*dbal.Index{}
	err := db.Select(&indexes, sql)
	if err != nil {
		return nil, err
	}

	// counting the type of indexes
	for _, index := range indexes {
		if index.Name == "PRIMARY" {
			index.Type = "primary"
		} else if index.Unique {
			index.Type = "unique"
		} else {
			index.Type = "index"
		}
	}
	return indexes, nil
}

// GetColumnListing get a table columns structure
func (grammarSQL SQL) GetColumnListing(dbName string, tableName string, db *sqlx.DB) ([]*dbal.Column, error) {
	selectColumns := []string{
		"TABLE_SCHEMA AS `db_name`",
		"TABLE_NAME AS `table_name`",
		"COLUMN_NAME AS `name`",
		"ORDINAL_POSITION AS `position`",
		"COLUMN_DEFAULT AS `default`",
		`CASE
			WHEN IS_NULLABLE = 'YES' THEN true
			WHEN IS_NULLABLE = "NO" THEN false
			ELSE false
		END AS ` + "`nullable`",
		`CASE
		   WHEN LOCATE('unsigned', COLUMN_TYPE) THEN true
		   ELSE false
		END AS` + "`unsigned`",
		"COLUMN_TYPE as `type_name`",
		"UPPER(DATA_TYPE) as `type`",
		"CHARACTER_MAXIMUM_LENGTH as `length`",
		"CHARACTER_OCTET_LENGTH as `octet_length`",
		"NUMERIC_PRECISION as `precision`",
		"NUMERIC_SCALE as `scale`",
		"DATETIME_PRECISION as `datetime_precision`",
		"CHARACTER_SET_NAME as `charset`",
		"COLLATION_NAME as `collation`",
		"COLUMN_KEY as `key`",
		`CASE
			WHEN COLUMN_KEY = 'PRI' THEN true
			ELSE false
		END AS ` + "`primary`",
		"EXTRA as `extra`",
		"COLUMN_COMMENT as `comment`",
	}
	sql := fmt.Sprintf(`
			SELECT %s
			FROM INFORMATION_SCHEMA.COLUMNS
			WHERE TABLE_SCHEMA = %s AND TABLE_NAME = %s;
		`,
		strings.Join(selectColumns, ","),
		grammarSQL.Quoter.VAL(dbName, db),
		grammarSQL.Quoter.VAL(tableName, db),
	)
	defer logger.Debug(logger.RETRIEVE, sql).TimeCost(time.Now())
	columns := []*dbal.Column{}
	err := db.Select(&columns, sql)
	if err != nil {
		return nil, err
	}

	// Cast the database data type to DBAL data type
	for _, column := range columns {
		typ, has := grammarSQL.FlipTypes[column.Type]
		if has {
			column.Type = typ
		}

		if column.Comment != nil {
			typ = grammarSQL.GetTypeFromComment(column.Comment)
			if typ != "" {
				column.Type = typ
			}
		}

		if column.Type == "enum" {
			re := regexp.MustCompile(`enum\('(.*)'\)`)
			matched := re.FindStringSubmatch(column.TypeName)
			if len(matched) == 2 {
				options := strings.Split(matched[1], "','")
				column.Option = options
			}
		}

		if utils.StringVal(column.Extra) == "auto_increment" {
			column.Extra = utils.StringPtr("AutoIncrement")
		}
	}
	return columns, nil
}

// CreateTable create a new table on the schema
func (grammarSQL SQL) CreateTable(table *dbal.Table, db *sqlx.DB) error {
	name := grammarSQL.Quoter.ID(table.TableName, db)
	sql := fmt.Sprintf("CREATE TABLE %s (\n", name)
	stmts := []string{}

	var primary *dbal.Primary = nil
	columns := []*dbal.Column{}
	indexes := []*dbal.Index{}
	cbCommands := []*dbal.Command{}

	// Commands
	// The commands must be:
	//    AddColumn(column *Column)    for adding a column
	//    ChangeColumn(column *Column) for modifying a colu
	//    RenameColumn(old string,new string)  for renaming a column
	//    DropColumn(name string)  for dropping a column
	//    CreateIndex(index *Index) for creating a index
	//    DropIndex( name string) for  dropping a index
	//    RenameIndex(old string,new string)  for renaming a index
	//    CreatePrimary for creating the primary key
	for _, command := range table.Commands {
		switch command.Name {
		case "AddColumn":
			columns = append(columns, command.Params[0].(*dbal.Column))
			cbCommands = append(cbCommands, command)
			break
		case "CreateIndex":
			indexes = append(indexes, command.Params[0].(*dbal.Index))
			cbCommands = append(cbCommands, command)
			break
		case "CreatePrimary":
			primary = command.Params[0].(*dbal.Primary)
			cbCommands = append(cbCommands, command)
			break
		}

	}

	// Columns
	for _, Column := range columns {
		stmts = append(stmts,
			grammarSQL.SQLAddColumn(db, Column),
		)
	}

	// Primary key
	if primary != nil {
		stmts = append(stmts,
			grammarSQL.SQLAddPrimary(db, primary),
		)
	}

	// indexes
	for _, index := range indexes {
		indexStmt := grammarSQL.SQLAddIndex(db, index)
		if indexStmt != "" {
			stmts = append(stmts, indexStmt)
		}
	}

	engine := utils.GetIF(table.Engine != "", "ENGINE "+table.Engine, "")
	charset := utils.GetIF(table.Charset != "", "DEFAULT CHARSET "+table.Charset, "")
	collation := utils.GetIF(table.Collation != "", "COLLATE="+table.Collation, "")

	sql = sql + strings.Join(stmts, ",\n")
	sql = sql + fmt.Sprintf(
		"\n) %s %s %s ROW_FORMAT=DYNAMIC",
		engine, charset, collation,
	)
	defer logger.Debug(logger.CREATE, sql).TimeCost(time.Now())
	_, err := db.Exec(sql)

	// Callback
	for _, cmd := range cbCommands {
		cmd.Callback(err)
	}

	return err
}

// DropTable a table from the schema.
func (grammarSQL SQL) DropTable(name string, db *sqlx.DB) error {
	sql := fmt.Sprintf("DROP TABLE %s", grammarSQL.Quoter.ID(name, db))
	defer logger.Debug(logger.DELETE, sql).TimeCost(time.Now())
	_, err := db.Exec(sql)
	return err
}

// DropTableIfExists if the table exists, drop it from the schema.
func (grammarSQL SQL) DropTableIfExists(name string, db *sqlx.DB) error {
	sql := fmt.Sprintf("DROP TABLE IF EXISTS %s", grammarSQL.Quoter.ID(name, db))
	defer logger.Debug(logger.DELETE, sql).TimeCost(time.Now())
	_, err := db.Exec(sql)
	return err
}

// Rename a table on the schema.
func (grammarSQL SQL) Rename(old string, new string, db *sqlx.DB) error {
	sql := fmt.Sprintf("ALTER TABLE %s RENAME %s", grammarSQL.Quoter.ID(old, db), grammarSQL.Quoter.ID(new, db))
	defer logger.Debug(logger.UPDATE, sql).TimeCost(time.Now())
	_, err := db.Exec(sql)
	return err
}

// AlterTable alter a table on the schema
func (grammarSQL SQL) AlterTable(table *dbal.Table, db *sqlx.DB) error {

	sql := fmt.Sprintf("ALTER TABLE %s ", grammarSQL.Quoter.ID(table.TableName, db))
	stmts := []string{}
	errs := []error{}

	// Commands
	// The commands must be:
	//    AddColumn(column *Column)    for adding a column
	//    ChangeColumn(column *Column) for modifying a colu
	//    RenameColumn(old string,new string)  for renaming a column
	//    DropColumn(name string)  for dropping a column
	//    CreateIndex(index *Index) for creating a index
	//    DropIndex(name string) for  dropping a index
	//    RenameIndex(old string,new string)  for renaming a index
	for _, command := range table.Commands {
		switch command.Name {
		case "AddColumn":
			column := command.Params[0].(*dbal.Column)
			stmt := "ADD " + grammarSQL.SQLAddColumn(db, column)
			stmts = append(stmts, sql+stmt)
			err := grammarSQL.ExecSQL(db, table, sql+stmt)
			if err != nil {
				errs = append(errs, fmt.Errorf("AddColumn: %s", err))
			}
			command.Callback(err)
			break
		case "ChangeColumn":
			column := command.Params[0].(*dbal.Column)
			stmt := "MODIFY " + grammarSQL.SQLAddColumn(db, column)
			stmts = append(stmts, sql+stmt)
			err := grammarSQL.ExecSQL(db, table, sql+stmt)
			if err != nil {
				errs = append(errs, fmt.Errorf("ChangeColumn %s: %s", column.Name, err))
			}
			command.Callback(err)
			break
		case "RenameColumn":
			old := command.Params[0].(string)
			new := command.Params[1].(string)
			column, has := table.ColumnMap[old]
			if !has {
				return fmt.Errorf("RenameColumn: The column %s not exists", old)
			}
			column.Name = new
			stmt := fmt.Sprintf("CHANGE COLUMN %s %s",
				grammarSQL.Quoter.ID(old, db),
				grammarSQL.SQLAddColumn(db, column),
			)
			stmts = append(stmts, sql+stmt)
			err := grammarSQL.ExecSQL(db, table, sql+stmt)
			if err != nil {
				errs = append(errs, fmt.Errorf("RenameColumn: %s", err))
			}
			command.Callback(err)
			break
		case "DropColumn":
			name := command.Params[0].(string)
			stmt := fmt.Sprintf("DROP COLUMN %s", grammarSQL.Quoter.ID(name, db))
			stmts = append(stmts, sql+stmt)
			err := grammarSQL.ExecSQL(db, table, sql+stmt)
			if err != nil {
				errs = append(errs, fmt.Errorf("DropColumn: %s", err))
			}
			break
		case "CreateIndex":
			index := command.Params[0].(*dbal.Index)
			stmt := "ADD " + grammarSQL.SQLAddIndex(db, index)
			stmts = append(stmts, sql+stmt)
			err := grammarSQL.ExecSQL(db, table, sql+stmt)
			if err != nil {
				errs = append(errs, fmt.Errorf("CreateIndex: %s", err))
			}
			break
		case "DropIndex":
			name := command.Params[0].(string)
			stmt := fmt.Sprintf("DROP INDEX %s", grammarSQL.Quoter.ID(name, db))
			stmts = append(stmts, sql+stmt)
			err := grammarSQL.ExecSQL(db, table, sql+stmt)
			if err != nil {
				errs = append(errs, fmt.Errorf("DropIndex: %s", err))
			}
			command.Callback(err)
			break
		case "RenameIndex":
			old := command.Params[0].(string)
			new := command.Params[1].(string)
			oldIndex := table.GetIndex(old)
			if oldIndex == nil {
				err := fmt.Errorf("RenameIndex: The index %s not found", old)
				errs = append(errs, err)
				command.Callback(err)
				break
			}

			stmt := fmt.Sprintf("DROP INDEX %s", grammarSQL.Quoter.ID(old, db))
			stmts = append(stmts, sql+stmt)
			err := grammarSQL.ExecSQL(db, table, sql+stmt)
			if err != nil {
				errs = append(errs, fmt.Errorf("RenameIndex: %s", err))
				command.Callback(err)
				break
			}

			newIndex := oldIndex
			newIndex.Name = new
			stmt = "ADD " + grammarSQL.SQLAddIndex(db, newIndex)
			stmts = append(stmts, sql+stmt)
			err = grammarSQL.ExecSQL(db, table, sql+stmt)
			if err != nil {
				errs = append(errs, fmt.Errorf("RenameIndex: %s", err))
			}
			command.Callback(err)

			// stmt := fmt.Sprintf("RENAME INDEX %s TO %s", grammarSQL.Quoter.ID(old, db), grammarSQL.Quoter.ID(new, db))
			// stmts = append(stmts, sql+stmt)
			// err := grammarSQL.ExecSQL(db, table, sql+stmt)
			// if err != nil {
			// 	errs = append(errs, err)
			// }
			// command.Callback(err)
			break
		}
	}

	defer logger.Debug(logger.CREATE, strings.Join(stmts, "\n")).TimeCost(time.Now())

	// Return Errors
	if len(errs) > 0 {
		message := ""
		for _, err := range errs {
			message = message + err.Error() + "\n"
		}
		return errors.New(message)
	}

	return nil
}

// ExecSQL execute sql then update table structure
func (grammarSQL SQL) ExecSQL(db *sqlx.DB, table *dbal.Table, sql string) error {
	_, err := db.Exec(sql)
	if err != nil {
		return err
	}
	// update table structure
	err = grammarSQL.GetTable(table, db)
	if err != nil {
		return err
	}
	return nil
}

// GetTypeFromComment Get the type name from comment
func (grammarSQL SQL) GetTypeFromComment(comment *string) string {
	if comment == nil {
		return ""
	}

	lines := strings.Split(*comment, "|")
	if len(lines) < 1 {
		return ""
	}

	re := regexp.MustCompile(`^T:([a-zA-Z]+)`)
	matched := re.FindStringSubmatch(lines[0])
	if len(matched) == 2 {
		return matched[1]
	}

	return ""
}
