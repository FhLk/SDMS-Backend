package usecase

import "sdms/internal/modules/topic/domain"

type CreateFieldInput struct {
	Label    string
	Type     domain.FieldType
	Required bool
	Position int
	Options  []domain.SelectOption
}

type UpdateFieldInput struct {
	Label    string
	Type     domain.FieldType
	Required bool
	Position int
	Options  []domain.SelectOption
}
