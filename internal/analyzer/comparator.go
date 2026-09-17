package analyzer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// Compare deterministically checks an observed JSON payload against an expected OpenAPI schema.
func Compare(endpoint, status string, schema *openapi3.Schema, observed any) *AnalysisResult {
	var findings []Finding

	if schema != nil {
		findings = compareRecursive("", schema, observed, true, false)
	}

	// De-duplicate findings that may arise from array iterations
	uniqueFindings := deduplicateFindings(findings)

	// Sort findings predictably by path, then kind
	sort.Slice(uniqueFindings, func(i, j int) bool {
		if uniqueFindings[i].Path == uniqueFindings[j].Path {
			return uniqueFindings[i].Kind < uniqueFindings[j].Kind
		}
		return uniqueFindings[i].Path < uniqueFindings[j].Path
	})

	hasBreaking := false
	for _, f := range uniqueFindings {
		if f.Breaking {
			hasBreaking = true
			break
		}
	}

	risk := CalculateOverallRisk(uniqueFindings)

	return &AnalysisResult{
		Endpoint: endpoint,
		Status:   status,
		Risk:     risk,
		Breaking: hasBreaking,
		Findings: uniqueFindings,
	}
}

func compareRecursive(path string, schema *openapi3.Schema, observed any, isRequired bool, strictAdditionalProps bool) []Finding {
	if schema == nil {
		return nil
	}

	var findings []Finding

	// Check null value
	if observed == nil {
		if !schema.PermitsNull() {
			sev, breaking := ClassifySeverity(KindNullableMismatch, isRequired, strictAdditionalProps)
			findings = append(findings, Finding{
				Path:         pathDisplay(path),
				Kind:         KindNullableMismatch,
				ExpectedType: schemaTypesString(schema),
				ActualType:   "null",
				Breaking:     breaking,
				Severity:     sev,
				Message:      "Field value is null but schema does not permit null",
			})
		}
		return findings
	}

	actualType, cleanVal := InferJSONType(observed)

	// Check type compatibility if expected type is defined
	if schema.Type != nil && !schema.Type.IsEmpty() {
		compatible := false
		for _, exp := range schema.Type.Slice() {
			if IsTypeCompatible(exp, actualType) {
				compatible = true
				break
			}
		}

		if !compatible {
			sev, breaking := ClassifySeverity(KindTypeMismatch, isRequired, strictAdditionalProps)
			expectedStr := strings.Join(schema.Type.Slice(), " | ")
			findings = append(findings, Finding{
				Path:         pathDisplay(path),
				Kind:         KindTypeMismatch,
				ExpectedType: expectedStr,
				ActualType:   actualType,
				ActualValue:  cleanVal,
				Breaking:     breaking,
				Severity:     sev,
				Message:      fmt.Sprintf("Type mismatch — expected %s, got %s", expectedStr, actualType),
			})
			return findings
		}
	}

	// Check enum constraint
	if len(schema.Enum) > 0 {
		matched := false
		for _, enumVal := range schema.Enum {
			if enumValuesMatch(enumVal, cleanVal) {
				matched = true
				break
			}
		}
		if !matched {
			sev, breaking := ClassifySeverity(KindEnumMismatch, isRequired, strictAdditionalProps)
			findings = append(findings, Finding{
				Path:          pathDisplay(path),
				Kind:          KindEnumMismatch,
				ExpectedValue: schema.Enum,
				ActualValue:   cleanVal,
				Breaking:      breaking,
				Severity:      sev,
				Message:       fmt.Sprintf("Value %v is not in documented enum: %v", cleanVal, schema.Enum),
			})
		}
	}

	// Check object properties
	if actualType == "object" {
		objMap, ok := observed.(map[string]any)
		if ok {
			isStrict := schema.AdditionalProperties.Has != nil && !*schema.AdditionalProperties.Has

			hasKey := make(map[string]bool, len(objMap))
			for k := range objMap {
				hasKey[k] = true
			}

			// 1. Required fields check
			for _, reqKey := range schema.Required {
				if !hasKey[reqKey] {
					subPath := joinPath(path, reqKey)
					sev, breaking := ClassifySeverity(KindRequiredFieldMissing, true, isStrict)
					var expType string
					if propRef := schema.Properties[reqKey]; propRef != nil && propRef.Value != nil {
						expType = schemaTypesString(propRef.Value)
					}
					findings = append(findings, Finding{
						Path:         subPath,
						Kind:         KindRequiredFieldMissing,
						ExpectedType: expType,
						Breaking:     breaking,
						Severity:     sev,
						Message:      "Required field is missing from response",
					})
				}
			}

			// 2. Documented optional fields check
			for propKey, propRef := range schema.Properties {
				if !hasKey[propKey] && !isFieldRequired(schema, propKey) {
					subPath := joinPath(path, propKey)
					sev, breaking := ClassifySeverity(KindDocumentedFieldMissing, false, isStrict)
					var expType string
					if propRef != nil && propRef.Value != nil {
						expType = schemaTypesString(propRef.Value)
					}
					findings = append(findings, Finding{
						Path:         subPath,
						Kind:         KindDocumentedFieldMissing,
						ExpectedType: expType,
						Breaking:     breaking,
						Severity:     sev,
						Message:      "Documented optional field is missing from response",
					})
				}
			}

			// 3. Unexpected fields check
			if len(schema.Properties) > 0 {
				for obsKey, obsVal := range objMap {
					if schema.Properties[obsKey] == nil {
						subPath := joinPath(path, obsKey)
						sev, breaking := ClassifySeverity(KindUnexpectedField, false, isStrict)
						valType, valClean := InferJSONType(obsVal)
						msg := "Field returned by API but not documented"
						if isStrict {
							msg = "Undocumented field returned when additionalProperties is false"
						}
						findings = append(findings, Finding{
							Path:        subPath,
							Kind:        KindUnexpectedField,
							ActualType:  valType,
							ActualValue: valClean,
							Breaking:    breaking,
							Severity:    sev,
							Message:     msg,
						})
					}
				}
			}

			// 4. Recursive inspection of present properties
			for propKey, propRef := range schema.Properties {
				if hasKey[propKey] && propRef != nil && propRef.Value != nil {
					subPath := joinPath(path, propKey)
					subReq := isFieldRequired(schema, propKey)
					childFindings := compareRecursive(subPath, propRef.Value, objMap[propKey], subReq, isStrict)
					findings = append(findings, childFindings...)
				}
			}
		}
	}

	// Check array elements
	if actualType == "array" {
		arrList, ok := observed.([]any)
		if ok && schema.Items != nil && schema.Items.Value != nil {
			itemPath := joinArrayPath(path)
			for _, item := range arrList {
				itemFindings := compareRecursive(itemPath, schema.Items.Value, item, false, strictAdditionalProps)
				findings = append(findings, itemFindings...)
			}
		}
	}

	return findings
}

func joinPath(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}

func joinArrayPath(parent string) string {
	if parent == "" {
		return "[]"
	}
	return parent + "[]"
}

func pathDisplay(path string) string {
	if path == "" {
		return "(root)"
	}
	return path
}

func isFieldRequired(schema *openapi3.Schema, fieldName string) bool {
	for _, req := range schema.Required {
		if req == fieldName {
			return true
		}
	}
	return false
}

func schemaTypesString(schema *openapi3.Schema) string {
	if schema == nil || schema.Type == nil {
		return ""
	}
	return strings.Join(schema.Type.Slice(), " | ")
}

func enumValuesMatch(enumVal, actualVal any) bool {
	if enumVal == actualVal {
		return true
	}
	// Normalization for integers vs floats in enums
	eStr := fmt.Sprintf("%v", enumVal)
	aStr := fmt.Sprintf("%v", actualVal)
	return eStr == aStr
}

func deduplicateFindings(findings []Finding) []Finding {
	seen := make(map[string]bool)
	var result []Finding

	for _, f := range findings {
		key := fmt.Sprintf("%s|%s|%s|%s|%v", f.Path, f.Kind, f.ExpectedType, f.ActualType, f.ActualValue)
		if !seen[key] {
			seen[key] = true
			result = append(result, f)
		}
	}

	return result
}
