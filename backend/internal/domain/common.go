package domain

// APIResponse standart API dönüş formatını belirler.
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Code    int         `json:"code,omitempty"`
}

type PaginationParams struct {
	Page  int `query:"page"`
	Limit int `query:"limit"`
}

func (p *PaginationParams) GetOffset() (limit int, offset int) {
	if p.Page <= 0 {
		p.Page = 1
	}
	if p.Limit <= 0 {
		p.Limit = 20 // Varsayılan limit
	}
	if p.Limit > 100 {
		p.Limit = 100
	}

	offset = (p.Page - 1) * p.Limit
	return p.Limit, offset
}
