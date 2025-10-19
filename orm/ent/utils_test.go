package entOrm_test

// import (
// 	".com/dev-viet/smm-document/api/common"
// 	"testing"

// 	"entgo.io/ent/dialect/sql"
// 	entOrm "github.com/tuan-dd/go-pkg/orm/ent"
// )

// func TestApplyProtoFilter(t *testing.T) {
// 	tests := []struct {
// 		name    string
// 		filters []*common.Filter
// 		columns []string
// 		query   string
// 		args    []any
// 	}{
// 		{
// 			name: "OR condition filters 1",
// 			filters: []*common.Filter{
// 				{Key: "age", Value: "25", WithOR: true, Op: common.OP_OP_LT},
// 				{Key: "status", Value: "active", WithOR: true, Op: common.OP_OP_GTE},
// 			},
// 			columns: []string{"status", "age", "name"},
// 			query:   "SELECT * FROM `test_table` WHERE `age` < ? OR `status` >= ?",
// 			args:    []any{"25", "active"},
// 		},
// 		{
// 			name: "OR condition filters 2",
// 			filters: []*common.Filter{
// 				{Key: "age", Value: "25", WithOR: true, Op: common.OP_OP_EQ},
// 				{Key: "status", Value: "active", WithOR: true, Op: common.OP_OP_EQ},
// 				{Key: "status", Value: "25", WithOR: false, Op: common.OP_OP_EQ},
// 			},
// 			columns: []string{"status", "age", "name"},
// 			query:   "SELECT * FROM `test_table` WHERE (`age` = ? OR `status` = ?) AND `status` = ?",
// 			args:    []any{"25", "active", "25"},
// 		},
// 		{
// 			name: "OR condition filters 3",
// 			filters: []*common.Filter{
// 				{Key: "status", Value: "25", WithOR: false, Op: common.OP_OP_EQ},    // index 1
// 				{Key: "age", Value: "25", WithOR: true, Op: common.OP_OP_EQ},        // index 1
// 				{Key: "status", Value: "active", WithOR: true, Op: common.OP_OP_EQ}, // index 2
// 			},
// 			columns: []string{"status", "age", "name"},
// 			query:   "SELECT * FROM `test_table` WHERE `status` = ? AND (`age` = ? OR `status` = ?)",
// 			args:    []any{"25", "25", "active"},
// 		},
// 		{
// 			name: "OR condition filters 4",
// 			filters: []*common.Filter{
// 				{Key: "status", Value: "25", WithOR: false, Op: common.OP_OP_EQ},          // index 1
// 				{Key: "age", Value: "25", WithOR: true, Op: common.OP_OP_EQ},              // index 1
// 				{Key: "status", Value: "active", WithOR: true, Op: common.OP_OP_EQ},       // index 2
// 				{Key: "role", Value: "1,2,3,4,5", WithOR: false, Op: common.OP_OP_NOT_IN}, // index 3
// 			},
// 			columns: []string{"status", "age", "name", "role"},
// 			query:   "SELECT * FROM `test_table` WHERE (`status` = ? AND (`age` = ? OR `status` = ?)) AND `role` NOT IN (?, ?, ?, ?, ?)",
// 			args:    []any{"25", "25", "active", "1", "2", "3", "4", "5"},
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			selector := sql.Select().From(sql.Table("test_table"))
// 			entOrm.ApplyProtoFilter(tt.filters, selector, tt.columns...)
// 			query, args := selector.Query()
// 			if query != tt.query {
// 				t.Errorf("Expected query %s, but got %s in test %s", tt.query, query, tt.name)
// 			}
// 			if len(args) != len(tt.args) {
// 				t.Fatalf("Expected %d args, but got %d args", len(tt.args), len(args))
// 			}
// 			for i, arg := range args {
// 				if arg != tt.args[i] {
// 					t.Errorf("Expected arg %v at index %d, but got %v", tt.args[i], i, arg)
// 				}
// 			}
// 		})
// 	}
// }
