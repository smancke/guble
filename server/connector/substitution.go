package connector

type substitution struct {
	FieldName string `json:"field"`
	OldValue  string `json:"old_value"`
	NewValue  string `json:"new_value"`
}

func (s *substitution) isValid() bool {
	return s.FieldName != "" && s.NewValue != "" && s.OldValue != ""
}
