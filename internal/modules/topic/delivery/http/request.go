package http

type CreateTopicRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type SelectOptionRequest struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type UpdateTopicRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsActive    bool   `json:"is_active"`
}

type CreateFieldRequest struct {
	Label    string                `json:"label"`
	Type     string                `json:"type"`
	Required bool                  `json:"required"`
	Position int                   `json:"position"`
	Options  []SelectOptionRequest `json:"options"`
}

type UpdateFieldRequest struct {
	Label    string                `json:"label"`
	Type     string                `json:"type"`
	Required bool                  `json:"required"`
	Position int                   `json:"position"`
	Options  []SelectOptionRequest `json:"options"`
}
