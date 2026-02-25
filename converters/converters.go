// Package converters provides functions to convert between different types.
// Copyright 2024-2026 George (earentir) Pantazis (https://earentir.dev)
// SPDX-License-Identifier: GPL-2.0-only
//
package converters

import (
	"fmt"
	"reflect"
	"strings"
)

// PrintFieldsByNames prints the values of the fields of a struct by their names.
func PrintFieldsByNames(input interface{}, fieldNames []string) {
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

// GetFieldValuesByNamesMap returns a map of field names to their values.
func GetFieldValuesByNamesMap(input interface{}, fieldNames []string) map[string]interface{} {
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

// GetFieldValuesByNamesArray returns an array of field values by their names.
func GetFieldValuesByNamesArray(input interface{}, fieldNames []string) []interface{} {
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

// ConvertValuesToStrings converts an array of interface{} values to an array of strings.
func ConvertValuesToStrings(values []interface{}) []string {
	var result []string

	for _, value := range values {
		result = append(result, fmt.Sprintf("%v", value))
	}

	return result
}

// ConvertIPToReverseDNS takes an IP address and converts it to a reverse DNS lookup string.
func ConvertIPToReverseDNS(ip string) string {
	// Split the IP address into its segments
	parts := strings.Split(ip, ".")

	// Check if the input is a valid IPv4 address (should have exactly 4 parts)
	if len(parts) != 4 {
		return "Invalid IP address"
	}

	// Reverse the order of the IP segments
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}

	// Join the reversed segments and append the ".in-addr.arpa" domain
	reverseDNS := strings.Join(parts, ".") + ".in-addr.arpa"

	return reverseDNS
}

// ConvertReverseDNSToIP takes a reverse DNS lookup string and converts it back to an IP address.
func ConvertReverseDNSToIP(reverseDNS string) string {
	// Split the reverse DNS string by "."
	parts := strings.Split(reverseDNS, ".")

	// Check if the input is valid (should have at least 4 parts before "in-addr" and "arpa")
	if len(parts) < 6 {
		return "Invalid input"
	}

	// Extract the first four segments which represent the reversed IP address
	ipParts := parts[:4]

	// Reverse the order of the extracted segments
	for i, j := 0, len(ipParts)-1; i < j; i, j = i+1, j-1 {
		ipParts[i], ipParts[j] = ipParts[j], ipParts[i]
	}

	// Join the segments back together to form the original IP address
	return strings.Join(ipParts, ".")
}
