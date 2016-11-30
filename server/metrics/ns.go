package metrics

const sep = "."

//NS is a namespace
type NS string

func (ns NS) NewInt(key string) Int {
	return NewInt(string(ns) + sep + key)
}

func (ns NS) NewMap(key string) Map {
	return NewMap(string(ns) + sep + key)
}

func (ns NS) NewNS(childKey string) NS {
	return NS(string(ns) + sep + childKey)
}
