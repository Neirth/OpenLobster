// Copyright (c) OpenLobster contributors.
// SPDX-License-Identifier: Apache-2.0

//! Minimal Serde implementation for GML (Graph Modelling Language).
//! Focuses on the subset required for OpenLobster memory: graph, node, edge blocks.

use serde::{de, ser, Deserialize, Serialize};
use std::fmt::{self, Display};

// ---------------------------------------------------------------------------
// Error Handling
// ---------------------------------------------------------------------------

#[derive(Debug)]
pub enum Error {
    Message(String),
    Unsupported,
    Eof,
    Syntax,
}

impl ser::Error for Error {
    fn custom<T: Display>(msg: T) -> Self { Error::Message(msg.to_string()) }
}

impl de::Error for Error {
    fn custom<T: Display>(msg: T) -> Self { Error::Message(msg.to_string()) }
}

impl Display for Error {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        match self {
            Error::Message(m) => write!(f, "{}", m),
            Error::Unsupported => write!(f, "unsupported GML operation"),
            Error::Eof => write!(f, "unexpected end of file"),
            Error::Syntax => write!(f, "GML syntax error"),
        }
    }
}

impl std::error::Error for Error {}

pub type Result<T> = std::result::Result<T, Error>;

// ---------------------------------------------------------------------------
// Serializer
// ---------------------------------------------------------------------------

pub struct Serializer {
    output: String,
    indent: usize,
}

pub fn to_string<T>(value: &T) -> Result<String>
where
    T: Serialize,
{
    let mut serializer = Serializer {
        output: String::new(),
        indent: 0,
    };
    value.serialize(&mut serializer)?;
    Ok(serializer.output)
}

impl<'a> ser::Serializer for &'a mut Serializer {
    type Ok = ();
    type Error = Error;
    type SerializeSeq = Self;
    type SerializeTuple = ser::Impossible<(), Error>;
    type SerializeTupleStruct = ser::Impossible<(), Error>;
    type SerializeTupleVariant = ser::Impossible<(), Error>;
    type SerializeMap = Self;
    type SerializeStruct = Self;
    type SerializeStructVariant = ser::Impossible<(), Error>;

    fn serialize_bool(self, v: bool) -> Result<()> {
        self.output.push_str(if v { "1" } else { "0" });
        Ok(())
    }

    fn serialize_i8(self, v: i8)    -> Result<()> { self.serialize_i64(v as i64) }
    fn serialize_i16(self, v: i16)  -> Result<()> { self.serialize_i64(v as i64) }
    fn serialize_i32(self, v: i32)  -> Result<()> { self.serialize_i64(v as i64) }
    fn serialize_i64(self, v: i64)  -> Result<()> {
        self.output.push_str(&v.to_string());
        Ok(())
    }

    fn serialize_u8(self, v: u8)    -> Result<()> { self.serialize_u64(v as u64) }
    fn serialize_u16(self, v: u16)  -> Result<()> { self.serialize_u64(v as u64) }
    fn serialize_u32(self, v: u32)  -> Result<()> { self.serialize_u64(v as u64) }
    fn serialize_u64(self, v: u64)  -> Result<()> {
        self.output.push_str(&v.to_string());
        Ok(())
    }

    fn serialize_f32(self, v: f32)  -> Result<()> { self.serialize_f64(v as f64) }
    fn serialize_f64(self, v: f64)  -> Result<()> {
        self.output.push_str(&v.to_string());
        Ok(())
    }

    fn serialize_char(self, v: char) -> Result<()> {
        self.serialize_str(&v.to_string())
    }

    fn serialize_str(self, v: &str) -> Result<()> {
        self.output.push('"');
        self.output.push_str(&v.replace('"', "\\\""));
        self.output.push('"');
        Ok(())
    }

    fn serialize_bytes(self, _v: &[u8]) -> Result<()> { Err(Error::Unsupported) }
    fn serialize_none(self)            -> Result<()> { Ok(()) }
    fn serialize_some<T>(self, value: &T) -> Result<()> where T: ?Sized + Serialize {
        value.serialize(self)
    }

    fn serialize_unit(self) -> Result<()> { Ok(()) }
    fn serialize_unit_struct(self, _name: &'static str) -> Result<()> { Ok(()) }
    fn serialize_unit_variant(self, _name: &'static str, _variant_index: u32, variant: &'static str) -> Result<()> {
        self.serialize_str(variant)
    }

    fn serialize_newtype_struct<T>(self, _name: &'static str, value: &T) -> Result<()> where T: ?Sized + Serialize {
        value.serialize(self)
    }

    fn serialize_newtype_variant<T>(self, _name: &'static str, _variant_index: u32, variant: &'static str, value: &T) -> Result<()> where T: ?Sized + Serialize {
        self.output.push_str(variant);
        self.output.push_str(" ");
        value.serialize(self)
    }

    fn serialize_seq(self, _len: Option<usize>) -> Result<Self::SerializeSeq> {
        Ok(self)
    }

    fn serialize_tuple(self, _len: usize) -> Result<Self::SerializeTuple> { Err(Error::Unsupported) }
    fn serialize_tuple_struct(self, _name: &'static str, _len: usize) -> Result<Self::SerializeTupleStruct> { Err(Error::Unsupported) }
    fn serialize_tuple_variant(self, _name: &'static str, _variant_index: u32, _variant: &'static str, _len: usize) -> Result<Self::SerializeTupleVariant> { Err(Error::Unsupported) }

    fn serialize_map(self, _len: Option<usize>) -> Result<Self::SerializeMap> { Ok(self) }

    fn serialize_struct(self, name: &'static str, _len: usize) -> Result<Self::SerializeStruct> {
        self.output.push_str(&"  ".repeat(self.indent));
        self.output.push_str(name);
        self.output.push_str(" [\n");
        self.indent += 1;
        Ok(self)
    }

    fn serialize_struct_variant(self, _name: &'static str, _variant_index: u32, _variant: &'static str, _len: usize) -> Result<Self::SerializeStructVariant> { Err(Error::Unsupported) }
}

impl<'a> ser::SerializeSeq for &'a mut Serializer {
    type Ok = ();
    type Error = Error;

    fn serialize_element<T>(&mut self, value: &T) -> Result<()> where T: ?Sized + Serialize {
        value.serialize(&mut **self)?;
        self.output.push('\n');
        Ok(())
    }
    fn end(self) -> Result<()> { Ok(()) }
}

impl<'a> ser::SerializeMap for &'a mut Serializer {
    type Ok = ();
    type Error = Error;
    fn serialize_key<T>(&mut self, key: &T) -> Result<()> where T: ?Sized + Serialize {
        self.output.push_str(&"  ".repeat(self.indent));
        key.serialize(&mut **self)?;
        self.output.push(' ');
        Ok(())
    }
    fn serialize_value<T>(&mut self, value: &T) -> Result<()> where T: ?Sized + Serialize {
        value.serialize(&mut **self)?;
        self.output.push('\n');
        Ok(())
    }
    fn end(self) -> Result<()> { Ok(()) }
}

impl<'a> ser::SerializeStruct for &'a mut Serializer {
    type Ok = ();
    type Error = Error;
    fn serialize_field<T>(&mut self, key: &'static str, value: &T) -> Result<()> where T: ?Sized + Serialize {
        // We only print the key if the value is NOT a struct/seq that will print its own name.
        // But since we can't easily know T, we use a different strategy:
        // In GML, every property is 'key value' or 'key [ ... ]'.
        // If we want it to be CLEAN, we'll let the primitive serializations NOT include the key,
        // and have serialize_field handle it for everything.
        
        // Wait, if it's a Vec<Struct>, each Struct will print its own name 'node ['.
        // We just need to avoid printing 'node' here.
        
        // Strategy: print key only if it's NOT 'node' or 'edge'? No, too specific.
        // Actually, for our Graph struct, 'node' and 'edge' are the only Vecs.
        
        if key == "node" || key == "edge" {
            // Skips printing the field name 'node' or 'edge' because the 
            // inner structs are already renamed to 'node' and 'edge'.
            value.serialize(&mut **self)?;
        } else {
            self.output.push_str(&"  ".repeat(self.indent));
            self.output.push_str(key);
            self.output.push(' ');
            value.serialize(&mut **self)?;
            self.output.push('\n');
        }
        Ok(())
    }
    fn end(self) -> Result<()> {
        self.indent -= 1;
        self.output.push_str(&"  ".repeat(self.indent));
        self.output.push_str("]");
        Ok(())
    }
}

// ---------------------------------------------------------------------------
// Deserializer (Simple non-conformant line-based, enough for our GML)
// ---------------------------------------------------------------------------

pub struct Deserializer<'de> {
    input: &'de str,
    current_key: Option<&'de str>,
    first_in_seq: bool,
}

pub fn from_str<'a, T>(s: &'a str) -> Result<T>
where
    T: Deserialize<'a>,
{
    let mut deserializer = Deserializer {
        input: s,
        current_key: None,
        first_in_seq: false,
    };
    T::deserialize(&mut deserializer)
}

impl<'de, 'a> de::Deserializer<'de> for &'a mut Deserializer<'de> {
    type Error = Error;

    fn deserialize_any<V>(self, _visitor: V) -> Result<V::Value> where V: de::Visitor<'de> { Err(Error::Unsupported) }
    fn deserialize_bool<V>(self, visitor: V) -> Result<V::Value> where V: de::Visitor<'de> {
        let s = self.next_token()?;
        visitor.visit_bool(s == "1")
    }
    fn deserialize_i8<V>(self, visitor: V) -> Result<V::Value> where V: de::Visitor<'de> { self.deserialize_i64(visitor) }
    fn deserialize_i16<V>(self, visitor: V) -> Result<V::Value> where V: de::Visitor<'de> { self.deserialize_i64(visitor) }
    fn deserialize_i32<V>(self, visitor: V) -> Result<V::Value> where V: de::Visitor<'de> { self.deserialize_i64(visitor) }
    fn deserialize_i64<V>(self, visitor: V) -> Result<V::Value> where V: de::Visitor<'de> {
        let s = self.next_token()?;
        visitor.visit_i64(s.parse().map_err(|_| Error::Syntax)?)
    }
    fn deserialize_u8<V>(self, visitor: V) -> Result<V::Value> where V: de::Visitor<'de> { self.deserialize_u64(visitor) }
    fn deserialize_u16<V>(self, visitor: V) -> Result<V::Value> where V: de::Visitor<'de> { self.deserialize_u64(visitor) }
    fn deserialize_u32<V>(self, visitor: V) -> Result<V::Value> where V: de::Visitor<'de> { self.deserialize_u64(visitor) }
    fn deserialize_u64<V>(self, visitor: V) -> Result<V::Value> where V: de::Visitor<'de> {
        let s = self.next_token()?;
        visitor.visit_u64(s.parse().map_err(|_| Error::Syntax)?)
    }
    fn deserialize_f32<V>(self, visitor: V) -> Result<V::Value> where V: de::Visitor<'de> { self.deserialize_f64(visitor) }
    fn deserialize_f64<V>(self, visitor: V) -> Result<V::Value> where V: de::Visitor<'de> {
        let s = self.next_token()?;
        visitor.visit_f64(s.parse().map_err(|_| Error::Syntax)?)
    }
    fn deserialize_char<V>(self, visitor: V) -> Result<V::Value> where V: de::Visitor<'de> { self.deserialize_string(visitor) }
    fn deserialize_str<V>(self, visitor: V) -> Result<V::Value> where V: de::Visitor<'de> { self.deserialize_string(visitor) }
    fn deserialize_string<V>(self, visitor: V) -> Result<V::Value> where V: de::Visitor<'de> {
        let s = self.next_token()?.trim_matches('"').replace("\\\"", "\"");
        visitor.visit_string(s)
    }
    fn deserialize_bytes<V>(self, _visitor: V) -> Result<V::Value> where V: de::Visitor<'de> { Err(Error::Unsupported) }
    fn deserialize_byte_buf<V>(self, _visitor: V) -> Result<V::Value> where V: de::Visitor<'de> { Err(Error::Unsupported) }
    fn deserialize_option<V>(self, visitor: V) -> Result<V::Value> where V: de::Visitor<'de> {
        visitor.visit_some(self)
    }
    fn deserialize_unit<V>(self, visitor: V) -> Result<V::Value> where V: de::Visitor<'de> { visitor.visit_unit() }
    fn deserialize_unit_struct<V>(self, _name: &'static str, visitor: V) -> Result<V::Value> where V: de::Visitor<'de> { visitor.visit_unit() }
    fn deserialize_newtype_struct<V>(self, _name: &'static str, visitor: V) -> Result<V::Value> where V: de::Visitor<'de> { visitor.visit_newtype_struct(self) }
    fn deserialize_seq<V>(self, visitor: V) -> Result<V::Value> where V: de::Visitor<'de> {
        let key = self.current_key;
        self.first_in_seq = true;
        visitor.visit_seq(CommaSeparated { de: self, end: Some("]"), key })
    }
    fn deserialize_tuple<V>(self, _len: usize, visitor: V) -> Result<V::Value> where V: de::Visitor<'de> { self.deserialize_seq(visitor) }
    fn deserialize_tuple_struct<V>(self, _name: &'static str, _len: usize, visitor: V) -> Result<V::Value> where V: de::Visitor<'de> { self.deserialize_seq(visitor) }
    fn deserialize_map<V>(self, visitor: V) -> Result<V::Value> where V: de::Visitor<'de> {
        visitor.visit_map(CommaSeparated { de: self, end: Some("]"), key: None })
    }
    fn deserialize_struct<V>(self, _name: &'static str, _fields: &'static [&'static str], visitor: V) -> Result<V::Value> where V: de::Visitor<'de> {
        let tok = self.next_token()?;
        if tok != "[" {
            let bracket = self.next_token()?;
            if bracket != "[" { return Err(Error::Syntax); }
        }
        visitor.visit_map(CommaSeparated { de: self, end: Some("]"), key: None })
    }
    fn deserialize_enum<V>(self, _name: &'static str, _variants: &'static [&'static str], _visitor: V) -> Result<V::Value> where V: de::Visitor<'de> { Err(Error::Unsupported) }
    fn deserialize_identifier<V>(self, visitor: V) -> Result<V::Value> where V: de::Visitor<'de> { self.deserialize_string(visitor) }
    fn deserialize_ignored_any<V>(self, visitor: V) -> Result<V::Value> where V: de::Visitor<'de> {
        let _ = self.next_token()?;
        visitor.visit_unit()
    }
}

impl<'de> Deserializer<'de> {
    fn de_peek(&self) -> Result<&'de str> {
        self.peek_token()
    }
    fn next_token(&mut self) -> Result<&'de str> {
        self.input = self.input.trim_start();
        if self.input.is_empty() { return Err(Error::Eof); }
        
        if self.input.starts_with('"') {
            // Find end of quoted string (simple, not handling escapes perfectly in tokenization)
            let mut escaped = false;
            let mut end = 1;
            for (i, c) in self.input[1..].chars().enumerate() {
                if escaped { escaped = false; }
                else if c == '\\' { escaped = true; }
                else if c == '"' {
                    end = i + 2;
                    break;
                }
            }
            let token = &self.input[..end];
            self.input = &self.input[end..];
            return Ok(token);
        }

        let end = self.input.find(|c: char| c.is_whitespace() || c == '[' || c == ']').unwrap_or(self.input.len());
        if end == 0 {
            // Must be [ or ]
            let token = &self.input[..1];
            self.input = &self.input[1..];
            return Ok(token);
        }
        let token = &self.input[..end];
        self.input = &self.input[end..];
        Ok(token)
    }

    fn peek_token(&self) -> Result<&'de str> {
        let s = self.input.trim_start();
        if s.is_empty() { return Err(Error::Eof); }
        
        if s.starts_with('"') {
            let mut escaped = false;
            let mut end = 1;
            for (i, c) in s[1..].chars().enumerate() {
                if escaped { escaped = false; }
                else if c == '\\' { escaped = true; }
                else if c == '"' {
                    end = i + 2;
                    break;
                }
            }
            return Ok(&s[..end]);
        }

        let end = s.find(|c: char| c.is_whitespace() || c == '[' || c == ']').unwrap_or(s.len());
        if end == 0 { return Ok(&s[..1]); }
        Ok(&s[..end])
    }
}

struct CommaSeparated<'a, 'de: 'a> {
    de: &'a mut Deserializer<'de>,
    end: Option<&'static str>,
    key: Option<&'de str>,
}

impl<'de, 'a> de::SeqAccess<'de> for CommaSeparated<'a, 'de> {
    type Error = Error;
    fn next_element_seed<T>(&mut self, seed: T) -> Result<Option<T::Value>> where T: de::DeserializeSeed<'de> {
        if let Some(expected_key) = self.key {
             if self.de.first_in_seq {
                 self.de.first_in_seq = false;
                 // First element: key was already consumed by MapAccess
             } else {
                 if self.de.de_peek().is_err() || self.de.de_peek()? != expected_key {
                     return Ok(None);
                 }
                 let _ = self.de.next_token()?; // consume repeating key
             }
        } else if let Some(end) = self.end {
            if self.de.de_peek().is_err() || self.de.de_peek()? == end {
                if !self.de.de_peek().is_err() { let _ = self.de.next_token()?; }
                return Ok(None);
            }
        }
        seed.deserialize(&mut *self.de).map(Some)
    }
}

impl<'de, 'a> de::MapAccess<'de> for CommaSeparated<'a, 'de> {
    type Error = Error;
    fn next_key_seed<K>(&mut self, seed: K) -> Result<Option<K::Value>> where K: de::DeserializeSeed<'de> {
        self.de.input = self.de.input.trim_start();
        if self.de.input.is_empty() { return Ok(None); }
        
        if let Some(end) = self.end {
            if self.de.de_peek()? == end {
                let _ = self.de.next_token()?;
                return Ok(None);
            }
        }
        
        let key = self.de.de_peek()?;
        self.de.current_key = Some(key);
        
        // If Serde is about to deserialize a Vec, it will call deserialize_seq.
        // If it's a repeating key, we want to stay in MapAccess only once for that key?
        // NO.
        seed.deserialize(&mut *self.de).map(Some)
    }
    fn next_value_seed<V>(&mut self, seed: V) -> Result<V::Value> where V: de::DeserializeSeed<'de> {
        let peek = self.de.peek_token()?;
        if peek == "[" {
            // Nested block
            seed.deserialize(&mut *self.de)
        } else {
            seed.deserialize(&mut *self.de)
        }
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;
    use serde::{Deserialize, Serialize};

    #[derive(Serialize, Deserialize, Debug, PartialEq)]
    #[serde(rename = "node")]
    #[serde(rename_all = "lowercase")]
    struct TestNode {
        id: u64,
        label: String,
    }

    #[derive(Serialize, Deserialize, Debug, PartialEq)]
    #[serde(rename = "edge")]
    #[serde(rename_all = "lowercase")]
    struct TestEdge {
        from: String,
        to: String,
        label: String,
    }

    #[derive(Serialize, Deserialize, Debug, PartialEq)]
    #[serde(rename = "graph")]
    #[serde(rename_all = "lowercase")]
    struct TestGraph {
        directed: i32,
        #[serde(rename = "node")]
        nodes: Vec<TestNode>,
        #[serde(rename = "edge")]
        edges: Vec<TestEdge>,
    }

    #[test]
    fn test_serialization() {
        let graph = TestGraph {
            directed: 1,
            nodes: vec![
                TestNode { id: 1, label: "A".to_string() },
                TestNode { id: 2, label: "B \"quoted\"".to_string() },
            ],
            edges: vec![
                TestEdge { from: "1".to_string(), to: "2".to_string(), label: "links".to_string() },
            ],
        };

        let gml = to_string(&graph).unwrap();
        // println!("DEBUG GML:\n{}", gml);
        assert!(gml.contains("node ["));
        assert!(gml.contains("id 1"));
        assert!(gml.contains("id 2"));
        assert!(gml.contains("label \"B \\\"quoted\\\"\""));
        assert!(gml.contains("edge ["));
        assert!(gml.contains("from \"1\""));
        assert!(gml.contains("to \"2\""));
        assert!(!gml.contains("node node [")); // Ensure no double naming
        assert!(!gml.contains("edge edge [")); // Ensure no double naming
    }

    #[test]
    fn test_deserialization() {
        let gml = "
        graph [
          directed 1
          node [
            id 10
            label \"Hello World\"
          ]
          node [
            id 20
            label \"Foo Bar\"
          ]
          edge [
            from \"10\"
            to \"20\"
            label \"friend\"
          ]
        ]";

        let graph: TestGraph = from_str(gml).unwrap();
        assert_eq!(graph.directed, 1);
        assert_eq!(graph.nodes.len(), 2);
        assert_eq!(graph.edges.len(), 1);
        assert_eq!(graph.edges[0].label, "friend");
        assert_eq!(graph.nodes[0].id, 10);
        assert_eq!(graph.nodes[0].label, "Hello World");
        assert_eq!(graph.nodes[1].id, 20);
        assert_eq!(graph.nodes[1].label, "Foo Bar");
    }

    #[test]
    fn test_roundtrip() {
        #[derive(Serialize, Deserialize, Debug, PartialEq)]
        #[serde(rename = "graph")]
        struct Root {
            directed: i32,
            #[serde(rename = "node")]
            nodes: Vec<TestNode>,
            #[serde(rename = "edge")]
            edges: Vec<TestEdge>,
        }

        let original = Root {
            directed: 1,
            nodes: vec![TestNode { id: 42, label: "Answer".to_string() }],
            edges: vec![TestEdge { from: "42".to_string(), to: "0".to_string(), label: "logic".to_string() }],
        };

        let gml = to_string(&original).unwrap();
        // println!("ROUNDTRIP GML:\n{}", gml);
        let decoded: Root = from_str(&gml).unwrap();
        assert_eq!(original.directed, decoded.directed);
        assert_eq!(original.nodes.len(), decoded.nodes.len());
        assert_eq!(original.edges.len(), decoded.edges.len());
        assert_eq!(original, decoded);
    }
}
