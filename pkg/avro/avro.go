package avro

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/linkedin/goavro/v2"
)

// SchemaRegistryClient handles communication with Schema Registry
type SchemaRegistryClient struct {
	baseURL    string
	httpClient *http.Client
}

// SchemaResponse represents the response from Schema Registry
type SchemaResponse struct {
	Subject string `json:"subject"`
	Version int    `json:"version"`
	ID      int    `json:"id"`
	Schema  string `json:"schema"`
}

// NewSchemaRegistryClient creates a new Schema Registry client
func NewSchemaRegistryClient(baseURL string) *SchemaRegistryClient {
	return &SchemaRegistryClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// RegisterSchema registers a schema and returns the schema ID
func (c *SchemaRegistryClient) RegisterSchema(subject, schema string) (int, error) {
	payload := map[string]interface{}{
		"schema": schema,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal schema payload: %w", err)
	}

	url := fmt.Sprintf("%s/subjects/%s/versions", c.baseURL, subject)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/vnd.schemaregistry.v1+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to register schema: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("schema registration failed with status %d: %s", resp.StatusCode, string(body))
	}

	var schemaResp SchemaResponse
	if err := json.NewDecoder(resp.Body).Decode(&schemaResp); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	return schemaResp.ID, nil
}

// GetSchemaByID retrieves a schema by ID
func (c *SchemaRegistryClient) GetSchemaByID(schemaID int) (string, error) {
	url := fmt.Sprintf("%s/schemas/ids/%d", c.baseURL, schemaID)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to get schema: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to get schema with status %d: %s", resp.StatusCode, string(body))
	}

	var schemaResp SchemaResponse
	if err := json.NewDecoder(resp.Body).Decode(&schemaResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return schemaResp.Schema, nil
}

// Serializer handles Avro serialization with Schema Registry integration
type Serializer struct {
	schemaRegistry *SchemaRegistryClient
	schemas        map[string]string
	schemaIDs      map[string]int
	mu             sync.RWMutex
}

// NewSerializer creates a new Avro serializer with Schema Registry support
func NewSerializer(schemaRegistryURL string) *Serializer {
	return &Serializer{
		schemaRegistry: NewSchemaRegistryClient(schemaRegistryURL),
		schemas:        make(map[string]string),
		schemaIDs:      make(map[string]int),
	}
}

// LoadSchema loads a schema from a file and registers it with Schema Registry
func (s *Serializer) LoadSchema(subject, filePath string) error {
	schemaBytes, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read schema file %s: %w", filePath, err)
	}

	schema := string(schemaBytes)

	// Register schema with Schema Registry
	schemaID, err := s.schemaRegistry.RegisterSchema(subject, schema)
	if err != nil {
		return fmt.Errorf("failed to register schema: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.schemas[subject] = schema
	s.schemaIDs[subject] = schemaID

	return nil
}

// GetSchema returns a schema by subject
func (s *Serializer) GetSchema(subject string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	schema, exists := s.schemas[subject]
	if !exists {
		return "", fmt.Errorf("schema not found for subject: %s", subject)
	}
	return schema, nil
}

// GetSchemaID returns a schema ID by subject
func (s *Serializer) GetSchemaID(subject string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	schemaID, exists := s.schemaIDs[subject]
	if !exists {
		return 0, fmt.Errorf("schema ID not found for subject: %s", subject)
	}
	return schemaID, nil
}

// Serialize serializes data using Avro schema with Confluent wire format
func (s *Serializer) Serialize(subject string, data map[string]interface{}) ([]byte, error) {
	// Get the schema ID for the subject
	schemaID, err := s.GetSchemaID(subject)
	if err != nil {
		return nil, fmt.Errorf("failed to get schema ID: %w", err)
	}

	// Get the schema for the subject
	schema, err := s.GetSchema(subject)
	if err != nil {
		return nil, fmt.Errorf("failed to get schema: %w", err)
	}

	// Create Avro codec
	codec, err := goavro.NewCodec(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to create codec: %w", err)
	}

	// Serialize the data
	avroData, err := codec.BinaryFromNative(nil, data)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize data: %w", err)
	}

	// Create the message with Confluent wire format
	// Format: [Magic Byte (0)] + [Schema ID (4 bytes, big endian)] + [Avro Data]
	message := make([]byte, 0, 1+4+len(avroData))

	// Magic byte (0)
	message = append(message, 0)

	// Schema ID (4 bytes, big endian)
	schemaIDBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(schemaIDBytes, uint32(schemaID))
	message = append(message, schemaIDBytes...)

	// Avro data
	message = append(message, avroData...)

	return message, nil
}

// Deserialize deserializes data using Avro schema with Confluent wire format
func (s *Serializer) Deserialize(data []byte) (map[string]interface{}, error) {
	if len(data) < 5 {
		return nil, fmt.Errorf("invalid message format: too short")
	}

	// Check magic byte
	if data[0] != 0 {
		return nil, fmt.Errorf("invalid magic byte: expected 0, got %d", data[0])
	}

	// Extract schema ID
	schemaID := int(binary.BigEndian.Uint32(data[1:5]))

	// Get schema from Schema Registry
	schema, err := s.schemaRegistry.GetSchemaByID(schemaID)
	if err != nil {
		return nil, fmt.Errorf("failed to get schema for ID %d: %w", schemaID, err)
	}

	// Create Avro codec
	codec, err := goavro.NewCodec(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to create codec: %w", err)
	}

	// Deserialize the data
	avroData := data[5:]
	native, _, err := codec.NativeFromBinary(avroData)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize data: %w", err)
	}

	// Convert to map
	result, ok := native.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("deserialized data is not a map")
	}

	return result, nil
}

// SerializeWithSchema serializes data with a specific schema string and registers it
func (s *Serializer) SerializeWithSchema(subject, schema string, data map[string]interface{}) ([]byte, error) {
	// Register schema with Schema Registry if not already registered
	s.mu.Lock()
	if _, exists := s.schemas[subject]; !exists {
		schemaID, err := s.schemaRegistry.RegisterSchema(subject, schema)
		if err != nil {
			s.mu.Unlock()
			return nil, fmt.Errorf("failed to register schema: %w", err)
		}
		s.schemas[subject] = schema
		s.schemaIDs[subject] = schemaID
	}
	s.mu.Unlock()

	// Create Avro codec
	codec, err := goavro.NewCodec(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to create codec: %w", err)
	}

	// Serialize the data
	avroData, err := codec.BinaryFromNative(nil, data)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize data: %w", err)
	}

	// Get schema ID
	schemaID, err := s.GetSchemaID(subject)
	if err != nil {
		return nil, fmt.Errorf("failed to get schema ID: %w", err)
	}

	// Create the message with Confluent wire format
	message := make([]byte, 0, 1+4+len(avroData))

	// Magic byte (0)
	message = append(message, 0)

	// Schema ID (4 bytes, big endian)
	schemaIDBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(schemaIDBytes, uint32(schemaID))
	message = append(message, schemaIDBytes...)

	// Avro data
	message = append(message, avroData...)

	return message, nil
}

// ValidateData validates data against a schema
func (s *Serializer) ValidateData(schema string, data map[string]interface{}) error {
	codec, err := goavro.NewCodec(schema)
	if err != nil {
		return fmt.Errorf("failed to create codec: %w", err)
	}

	// Try to serialize to validate
	_, err = codec.BinaryFromNative(nil, data)
	if err != nil {
		return fmt.Errorf("data validation failed: %w", err)
	}

	return nil
}

// ReadSchemaFile reads a schema from a file
func ReadSchemaFile(reader io.Reader) (string, error) {
	var buf bytes.Buffer
	_, err := io.Copy(&buf, reader)
	if err != nil {
		return "", fmt.Errorf("failed to read schema file: %w", err)
	}
	return buf.String(), nil
}
