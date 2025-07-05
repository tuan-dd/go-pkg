package pagination

// filter and sort utilities for pagination with default max is 100
func PaginationOpts(numArr ...int32) (take, skip int) {
	if len(numArr) < 2 {
		return 100, 0
	}
	if len(numArr) == 2 {
		numArr = append(numArr, 100)
	}

	limit := min(numArr[1], max(numArr[2], 1))

	page := max(numArr[0], 0)
	return int(limit), (int(page) * int(limit))
}
