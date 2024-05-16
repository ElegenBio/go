package utils

func WrapToList(object interface{}) []interface{} {
	objects := make([]interface{}, 1)
	objects[0] = object
	return objects
}
