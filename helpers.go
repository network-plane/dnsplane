package main

import (
	"fmt"
	"reflect"
)

func printFieldsByNames(input interface{}, fieldNames []string) {
	val := reflect.ValueOf(input)
	typ := val.Type()

	if typ.Kind() != reflect.Struct {
		fmt.Println("Expected a struct")
		return
	}

	for _, fieldName := range fieldNames {
		field := val.FieldByName(fieldName)
		if field.IsValid() {
			fmt.Printf("%s: %v\n", fieldName, field)
		} else {
			fmt.Printf("%s: field not found\n", fieldName)
		}
	}
}

func getFieldValuesByNamesMap(input interface{}, fieldNames []string) map[string]interface{} {
	val := reflect.ValueOf(input)
	typ := val.Type()
	result := make(map[string]interface{})

	if typ.Kind() != reflect.Struct {
		fmt.Println("Expected a struct")
		return nil
	}

	for _, fieldName := range fieldNames {
		field := val.FieldByName(fieldName)
		if field.IsValid() {
			result[fieldName] = field.Interface()
		} else {
			result[fieldName] = nil
		}
	}

	return result
}

func getFieldValuesByNamesArray(input interface{}, fieldNames []string) []interface{} {
	val := reflect.ValueOf(input)
	typ := val.Type()
	var result []interface{}

	if typ.Kind() != reflect.Struct {
		fmt.Println("Expected a struct")
		return nil
	}

	for _, fieldName := range fieldNames {
		field := val.FieldByName(fieldName)
		if field.IsValid() {
			result = append(result, field.Interface())
		} else {
			result = append(result, nil)
		}
	}

	return result
}

func convertValuesToStrings(values []interface{}) []string {
	var result []string

	for _, value := range values {
		result = append(result, fmt.Sprintf("%v", value))
	}

	return result
}
