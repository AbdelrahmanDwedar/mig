package sqlgen

import (
	"encoding/json"
	"fmt"
)

// BuildStatement dispatches a single JSON op (identified by its "op" field)
// to the corresponding Dialect builder method, returning the native SQL
// statement(s) it expands to. The "sql" op is the escape hatch and bypasses
// the dialect entirely.
func BuildStatement(dialect Dialect, raw json.RawMessage) ([]string, error) {
	var envelope opEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("invalid migration op: %w", err)
	}

	switch envelope.Op {
	case "sql":
		var op SQLOp
		if err := json.Unmarshal(raw, &op); err != nil {
			return nil, fmt.Errorf("invalid sql op: %w", err)
		}
		return []string{op.Query}, nil

	case "create_table":
		var op CreateTableOp
		if err := unmarshalOp(raw, &op); err != nil {
			return nil, err
		}
		return dialect.CreateTable(op)

	case "drop_table":
		var op DropTableOp
		if err := unmarshalOp(raw, &op); err != nil {
			return nil, err
		}
		return dialect.DropTable(op)

	case "rename_table":
		var op RenameTableOp
		if err := unmarshalOp(raw, &op); err != nil {
			return nil, err
		}
		return dialect.RenameTable(op)

	case "add_column":
		var op AddColumnOp
		if err := unmarshalOp(raw, &op); err != nil {
			return nil, err
		}
		return dialect.AddColumn(op)

	case "drop_column":
		var op DropColumnOp
		if err := unmarshalOp(raw, &op); err != nil {
			return nil, err
		}
		return dialect.DropColumn(op)

	case "rename_column":
		var op RenameColumnOp
		if err := unmarshalOp(raw, &op); err != nil {
			return nil, err
		}
		return dialect.RenameColumn(op)

	case "alter_column":
		var op AlterColumnOp
		if err := unmarshalOp(raw, &op); err != nil {
			return nil, err
		}
		return dialect.AlterColumn(op)

	case "add_index":
		var op AddIndexOp
		if err := unmarshalOp(raw, &op); err != nil {
			return nil, err
		}
		return dialect.AddIndex(op)

	case "drop_index":
		var op DropIndexOp
		if err := unmarshalOp(raw, &op); err != nil {
			return nil, err
		}
		return dialect.DropIndex(op)

	case "add_foreign_key":
		var op AddForeignKeyOp
		if err := unmarshalOp(raw, &op); err != nil {
			return nil, err
		}
		return dialect.AddForeignKey(op)

	case "drop_foreign_key":
		var op DropForeignKeyOp
		if err := unmarshalOp(raw, &op); err != nil {
			return nil, err
		}
		return dialect.DropForeignKey(op)

	case "add_constraint":
		var op AddConstraintOp
		if err := unmarshalOp(raw, &op); err != nil {
			return nil, err
		}
		return dialect.AddConstraint(op)

	case "drop_constraint":
		var op DropConstraintOp
		if err := unmarshalOp(raw, &op); err != nil {
			return nil, err
		}
		return dialect.DropConstraint(op)

	case "":
		return nil, fmt.Errorf(`migration op is missing required "op" field`)

	default:
		return nil, fmt.Errorf("unknown migration op: %q", envelope.Op)
	}
}

func unmarshalOp(raw json.RawMessage, v interface{}) error {
	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("invalid op payload: %w", err)
	}
	return nil
}
