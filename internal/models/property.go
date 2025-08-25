package models

type Property struct {
	ID    string
	Value string
}

func (p *Property) TableName() string {
	return "properties"
}
