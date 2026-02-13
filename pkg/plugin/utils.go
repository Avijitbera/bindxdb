package plugin

import "strings"

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
