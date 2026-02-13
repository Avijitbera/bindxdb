package plugin

import (
	"fmt"
	"strings"
)

func NewBasicFilter(column string, operator FilterOperator, value interface{}) Filter {
	return &BasicFilter{
		Column:   column,
		Operator: operator,
		Value:    value,
	}
}

func NewCompositeFilter(filters []Filter, and bool) Filter {
	return &CompositeFilter{
		Filters: filters,
		And:     and,
	}
}

func DataTypeToString(dt DataType) string {
	switch dt {
	case TypeInteger:
		return "INTEGER"
	case TypeBigInt:
		return "BIGINT"
	case TypeFloat:
		return "FLOAT"
	case TypeDouble:
		return "DOUBLE"
	case TypeDecimal:
		return "DECIMAL"
	case TypeBoolean:
		return "BOOLEAN"
	case TypeVarchar:
		return "VARCHAR"
	case TypeText:
		return "TEXT"
	case TypeBlob:
		return "BLOB"
	case TypeTimestamp:
		return "TIMESTAMP"
	case TypeDate:
		return "DATE"
	case TypeTime:
		return "TIME"
	case TypeJSON:
		return "JSON"
	case TypeUUID:
		return "UUID"
	default:
		return "UNKNOWN"
	}
}

func StringToDataType(s string) DataType {
	switch strings.ToUpper(s) {
	case "INTEGER", "INT":
		return TypeInteger
	case "BIGINT":
		return TypeBigInt
	case "FLOAT", "REAL":
		return TypeFloat
	case "DOUBLE", "DOUBLE PRECISION":
		return TypeDouble
	case "DECIMAL", "NUMERIC":
		return TypeDecimal
	case "BOOLEAN", "BOOL":
		return TypeBoolean
	case "VARCHAR", "CHARACTER VARYING":
		return TypeVarchar
	case "TEXT":
		return TypeText
	case "BLOB", "BYTEA":
		return TypeBlob
	case "TIMESTAMP", "DATETIME":
		return TypeTimestamp
	case "DATE":
		return TypeDate
	case "TIME":
		return TypeTime
	case "JSON", "JSONB":
		return TypeJSON
	case "UUID":
		return TypeUUID
	default:
		return TypeUnknown
	}
}

func IndexTypeToString(it IndexType) string {
	switch it {
	case IndexTypeBTree:
		return "BTREE"
	case IndexTypeHash:
		return "HASH"
	case IndexTypeRTree:
		return "RTREE"
	case IndexTypeFullText:
		return "FULLTEXT"
	case IndexTypeBitmap:
		return "BITMAP"
	default:
		return "UNKNOWN"
	}
}

func OperatorToString(op FilterOperator) string {
	switch op {
	case OperatorEquals:
		return "="
	case OperatorNotEqual:
		return "!="
	case OperatorGreaterThen:
		return ">"
	case OperatorGreaterThenOrEqual:
		return ">="
	case OperatorLessThen:
		return "<"
	case OperatorLessThenOrEqual:
		return "<="
	case OperatorLike:
		return "LIKE"
	case OperatorIn:
		return "IN"
	case OperatorIsNull:
		return "IS NULL"
	case OperatorIsNotNull:
		return "IS NOT NULL"
	case OperatorBetween:
		return "BETWEEN"
	default:
		return "UNKNOWN"
	}
}

func ValidateColumnDef(col *ColumnDef) error {
	if col.Name == "" {
		return fmt.Errorf("column name cannot be empty")
	}
	if col.Type == TypeUnknown {
		return fmt.Errorf("column type cannot be unknown")
	}

	if col.PrimaryKey && col.Nullable {
		return fmt.Errorf("primary key column cannot be nullable")
	}
	return nil
}

func ValidateTableSchema(schema *TableSchema) error {
	if schema.Name == "" {
		return fmt.Errorf("table name cannot be empty")
	}
	if len(schema.Columns) == 0 {
		return fmt.Errorf("table must have at least one column")
	}
	primaryKeyCount := 0
	columnNames := make(map[string]bool)
	for i, col := range schema.Columns {
		if err := ValidateColumnDef(&col); err != nil {
			return fmt.Errorf("column %d: %w", i, err)
		}
		if columnNames[col.Name] {
			return fmt.Errorf("duplicate column name: %s", col.Name)
		}
		columnNames[col.Name] = true
		if col.PrimaryKey {
			primaryKeyCount++
		}
	}
	for i, idx := range schema.Indexes {
		for j, colName := range idx.Columns {
			if !columnNames[colName] {
				return fmt.Errorf("index %d, column %d: column %s does not exist", i, j, colName)
			}
		}
	}
	return nil
}
