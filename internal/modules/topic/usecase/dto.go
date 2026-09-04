package usecase

import "sdms/internal/modules/topic/domain"

type CreateFieldInput struct {
	Label    string
	Type     domain.FieldType
	Required bool
	Position int
}

type UpdateFieldInput struct {
	Label    string
	Type     domain.FieldType
	Required bool
	Position int
}
