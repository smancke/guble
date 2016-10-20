package fcm

type jsonError struct {
	json string
}

func (e *jsonError) Error() string {
	return e.json
}
