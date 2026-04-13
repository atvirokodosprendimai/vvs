package domain

type Pagination struct {
	Page     int
	PageSize int
}

func NewPagination(page, pageSize int) Pagination {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 25
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return Pagination{Page: page, PageSize: pageSize}
}

func (p Pagination) Offset() int {
	return (p.Page - 1) * p.PageSize
}

func (p Pagination) TotalPages(total int64) int {
	if total == 0 {
		return 1
	}
	pages := int(total) / p.PageSize
	if int(total)%p.PageSize > 0 {
		pages++
	}
	return pages
}
