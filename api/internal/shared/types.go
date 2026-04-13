package shared

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// JSON is a JSONB-compatible type for PostgreSQL.
type JSON []byte

func (j JSON) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return j, nil
}

func (j *JSON) UnmarshalJSON(data []byte) error {
	if j == nil {
		return fmt.Errorf("shared.JSON: UnmarshalJSON on nil pointer")
	}
	*j = append((*j)[0:0], data...)
	return nil
}

func (j JSON) Value() (driver.Value, error) {
	if len(j) == 0 {
		return "{}", nil
	}
	return string(j), nil
}

func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = JSON("{}")
		return nil
	}
	switch v := value.(type) {
	case []byte:
		*j = append((*j)[0:0], v...)
	case string:
		*j = []byte(v)
	default:
		return fmt.Errorf("shared.JSON: unsupported type %T", value)
	}
	return nil
}

// GormDataType tells GORM to use jsonb column type.
func (JSON) GormDataType() string {
	return "jsonb"
}

// MustMarshal marshals v to JSON, panics on error. For use in tests/init only.
func MustMarshal(v interface{}) JSON {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return JSON(b)
}

// Response is the standard API response envelope.
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

// Meta contains pagination metadata.
type Meta struct {
	Total  int64  `json:"total,omitempty"`
	Cursor string `json:"cursor,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

// APIError represents a structured API error.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// OK returns a successful response.
func OK(data interface{}) Response {
	return Response{Success: true, Data: data}
}

// OKWithMeta returns a successful response with pagination metadata.
func OKWithMeta(data interface{}, meta *Meta) Response {
	return Response{Success: true, Data: data, Meta: meta}
}

// Fail returns an error response.
func Fail(code, message string) Response {
	return Response{Success: false, Error: &APIError{Code: code, Message: message}}
}
