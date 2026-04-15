// Copyright (c) OpenLobster contributors. See LICENSE for details.
// SPDX-License-Identifier: Apache-2.0

//! Schema validation for [`crate::protocol::PluginInfo::schema`].
//!
//! Implements the OpenLobster plugin schema contract specification.
//! The Go side (`utils/schema_contract`) is the canonical implementation of
//! the same contract; a plugin whose schema passes this check will also pass
//! the host-side validation in the backend and the smoke validator.

/// Valid JSON Schema primitive types accepted by the backend.
const VALID_TYPES: &[&str] =
    &["string", "number", "integer", "boolean", "object", "array"];

/// Validates the JSON Schema fragment stored in [`crate::protocol::PluginInfo::schema`].
///
/// Rules:
/// 1. The root `type` field, when present, must be `"object"`.
/// 2. Every entry in the `required` array must appear as a key in `properties`.
/// 3. Every property that declares a `type` must use a valid JSON Schema type.
///
/// Returns `Ok(())` when the schema is valid, or `Err` with a human-readable
/// description of the first violation found.
pub fn validate_schema(schema: &serde_json::Value) -> Result<(), String> {
    // Null or absent schema is fine — treated as an empty object schema.
    if schema.is_null() {
        return Ok(());
    }

    let obj = match schema.as_object() {
        Some(o) => o,
        None => return Err("schema must be a JSON object".to_string()),
    };

    // Rule 1: root type, if present, must be "object".
    if let Some(root_type) = obj.get("type") {
        match root_type.as_str() {
            Some("object") | None => {}
            Some(t) => {
                return Err(format!(
                    "schema root type must be \"object\", got \"{}\"",
                    t
                ))
            }
        }
    }

    // Collect property keys (may be absent).
    let properties: std::collections::HashSet<&str> = obj
        .get("properties")
        .and_then(|p| p.as_object())
        .map(|p| p.keys().map(String::as_str).collect())
        .unwrap_or_default();

    // Rule 2: every required field must exist in properties.
    if let Some(required) = obj.get("required") {
        let arr = required
            .as_array()
            .ok_or_else(|| "schema \"required\" must be an array".to_string())?;
        for item in arr {
            let field = item
                .as_str()
                .ok_or_else(|| "schema \"required\" entries must be strings".to_string())?;
            if !properties.contains(field) {
                return Err(format!(
                    "schema \"required\" field \"{}\" is not declared in \"properties\"",
                    field
                ));
            }
        }
    }

    // Rule 3: validate property types.
    if let Some(props) = obj.get("properties").and_then(|p| p.as_object()) {
        for (key, prop) in props {
            if let Some(type_val) = prop.get("type") {
                let type_str = type_val.as_str().ok_or_else(|| {
                    format!("property \"{}\" type must be a string", key)
                })?;
                if !VALID_TYPES.contains(&type_str) {
                    return Err(format!(
                        "property \"{}\" has invalid type \"{}\"; valid types: {}",
                        key,
                        type_str,
                        VALID_TYPES.join(", ")
                    ));
                }
            }
        }
    }

    Ok(())
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn null_schema_is_valid() {
        assert!(validate_schema(&json!(null)).is_ok());
    }

    #[test]
    fn empty_object_schema_is_valid() {
        assert!(validate_schema(&json!({})).is_ok());
    }

    #[test]
    fn object_type_is_valid() {
        assert!(validate_schema(&json!({"type": "object"})).is_ok());
    }

    #[test]
    fn non_object_root_type_is_rejected() {
        let err = validate_schema(&json!({"type": "string"})).unwrap_err();
        assert!(err.contains("root type"), "error: {}", err);
    }

    #[test]
    fn required_field_missing_from_properties_is_rejected() {
        let schema = json!({
            "type": "object",
            "properties": { "api_key": { "type": "string" } },
            "required": ["api_key", "missing_field"]
        });
        let err = validate_schema(&schema).unwrap_err();
        assert!(err.contains("missing_field"), "error: {}", err);
    }

    #[test]
    fn valid_full_schema_passes() {
        let schema = json!({
            "type": "object",
            "properties": {
                "api_key":  { "type": "string" },
                "max_tokens": { "type": "integer" },
                "enabled":  { "type": "boolean" }
            },
            "required": ["api_key"]
        });
        assert!(validate_schema(&schema).is_ok());
    }

    #[test]
    fn invalid_property_type_is_rejected() {
        let schema = json!({
            "type": "object",
            "properties": {
                "count": { "type": "float" }
            }
        });
        let err = validate_schema(&schema).unwrap_err();
        assert!(err.contains("float"), "error: {}", err);
    }

    #[test]
    fn non_object_schema_is_rejected() {
        let err = validate_schema(&json!("not an object")).unwrap_err();
        assert!(err.contains("JSON object"), "error: {}", err);
    }
}
