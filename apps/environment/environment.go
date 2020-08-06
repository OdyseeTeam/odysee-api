package environment

type Environment struct {
	objects map[string]interface{}
}

func (e *Environment) Add(k string, value interface{}) {
	e.objects[k] = value
}

func (e *Environment) Get(k string) interface{} {
	return e.objects[k]
}
