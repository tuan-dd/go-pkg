package entOrm

// type Op int32

// const (
// 	OP_LT          Op = 0
// 	OP_LTE         Op = 1
// 	OP_GT          Op = 2
// 	OP_GTE         Op = 3
// 	OP_EQ          Op = 4
// 	OP_NEQ         Op = 5
// 	OP_LIKE        Op = 6
// 	OP_IS_NULL     Op = 7
// 	OP_IS_NOT_NULL Op = 8
// 	OP_IN          Op = 9
// 	OP_NOT_IN      Op = 10
// )

// func ParseSort(columns []string, table string, sorts []*common.SortMethod) func(s *sql.Selector) {
// 	var values []string
// 	if len(sorts) == 0 {
// 		col := fmt.Sprintf("`%s`.`%s`", table, "id")
// 		values = append(values, sql.Desc(col))

// 	}
// 	for _, sort := range sorts {
// 		key := utils.ToSnakeCase(sort.Key)
// 		if !slices.Contains(columns, key) {
// 			return nil
// 		}

// 		col := fmt.Sprintf("`%s`.`%s`", table, key)
// 		switch sort.Value {
// 		case common.SortType_ASC:
// 			values = append(values, sql.Asc(col))
// 		case common.SortType_DESC:
// 			values = append(values, sql.Desc(col))
// 		default:
// 			return nil
// 		}
// 	}

// 	return func(s *sql.Selector) {
// 		if len(values) > 0 {
// 			s.OrderBy(values...)
// 		}
// 	}
// }

// func ApplyProtoFilter(filters []*common.Filter, selector *sql.Selector, columns ...string) {
// 	if len(filters) == 0 {
// 		return
// 	}
// 	if len(columns) == 0 {
// 		columns = selector.Columns()
// 	}

// 	columnMap := make(map[string]bool, len(columns))
// 	for _, col := range columns {
// 		columnMap[col] = true
// 	}

// 	for i := 0; i < len(filters); i++ {
// 		f := filters[i]
// 		if f.WithOR {
// 			filterQuery := []*sql.Predicate{}
// 			for j := i; j < len(filters); j++ {
// 				f := filters[j]
// 				f.Key = utils.ToSnakeCase(f.Key)
// 				if !columnMap[f.Key] {
// 					continue
// 				}
// 				// stop if the next filter is not an OR condition
// 				if !f.WithOR {
// 					break
// 				}
// 				filterQuery = append(filterQuery, buildFilter(f, common.OP(f.Op)))
// 				i = j
// 			}
// 			selector.Where(sql.Or(filterQuery...))
// 			continue
// 		}

// 		f.Key = utils.ToSnakeCase(f.Key)
// 		if !columnMap[f.Key] {
// 			continue
// 		}

// 		selector.Where(buildFilter(f, common.OP(f.Op)))
// 	}
// }

// func buildFilter(f *common.Filter, op common.OP) *sql.Predicate {
// 	switch op {
// 	case common.OP_OP_LT:
// 		return sql.LT(f.Key, f.Value)
// 	case common.OP_OP_LTE:
// 		return sql.LTE(f.Key, f.Value)
// 	case common.OP_OP_GT:
// 		return sql.GT(f.Key, f.Value)
// 	case common.OP_OP_GTE:
// 		return sql.GTE(f.Key, f.Value)
// 	case common.OP_OP_EQ:
// 		return sql.EQ(f.Key, f.Value)
// 	case common.OP_OP_NEQ:
// 		return sql.NEQ(f.Key, f.Value)
// 	case common.OP_OP_LIKE:
// 		return sql.ContainsFold(f.Key, f.Value)
// 	case common.OP_OP_IS_NULL:
// 		return sql.IsNull(f.Key)
// 	case common.OP_OP_IS_NOT_NULL:
// 		return sql.NotNull(f.Key)
// 	case common.OP_OP_IN:
// 		parts := ParseInFilterValues(f)
// 		if len(parts) == 1 {
// 			return sql.EQ(f.Key, parts[0])
// 		} else if len(parts) > 1 {
// 			return sql.In(f.Key, parts...)
// 		}
// 	case common.OP_OP_NOT_IN:
// 		parts := ParseInFilterValues(f)
// 		if len(parts) == 1 {
// 			return sql.NEQ(f.Key, parts[0])
// 		} else if len(parts) > 1 {
// 			return sql.NotIn(f.Key, parts...)
// 		}
// 	default:
// 		return sql.ContainsFold(f.Key, f.Value)
// 	}
// 	return nil
// }

// func ParseInFilterValues(f *common.Filter) []any {
// 	return lo.FilterMap(strings.Split(f.Value, ","), func(part string, _ int) (any, bool) {
// 		part = strings.TrimSpace(part)
// 		return part, part != ""
// 	})
// }

// func DefaultPagination(p *common.PaginationReq) *common.PaginationReq {
// 	if p == nil {
// 		return &common.PaginationReq{
// 			PageIndex: 0,
// 			PageSize:  25,
// 		}
// 	}
// 	return p
// }
