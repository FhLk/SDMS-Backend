package http

type CreateTopicRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type UpdateTopicRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsActive    bool   `json:"is_active"`
}

type CreateFieldRequest struct {
	Label    string `json:"label"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
	Position int    `json:"position"`
}
