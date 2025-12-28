package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/exec"
	"github.com/testcontainers/testcontainers-go/wait"
)

const testPort = "18090"

const (
	testSuperuserEmail    = "test@test.com"
	testSuperuserPassword = "testpassword123"
)

type PocketBaseContainer struct {
	testcontainers.Container
	URI   string
	Token string
}

func setupPocketBase(ctx context.Context, t *testing.T) (*PocketBaseContainer, error) {
	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    ".",
			Dockerfile: "Dockerfile",
		},
		ExposedPorts: []string{testPort + ":8090/tcp"},
		Env: map[string]string{
			"POCKETBASE_TEST_MODE": "true",
		},
		WaitingFor: wait.ForHTTP("/api/health").WithPort("8090/tcp").WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, err
	}

	host, err := container.Host(ctx)
	if err != nil {
		return nil, err
	}

	mappedPort, err := container.MappedPort(ctx, "8090/tcp")
	if err != nil {
		return nil, err
	}

	uri := fmt.Sprintf("http://%s:%s", host, mappedPort.Port())

	// Create superuser via CLI
	_, _, err = container.Exec(ctx, []string{
		"/pb/pocketbase", "superuser", "upsert", testSuperuserEmail, testSuperuserPassword,
	}, exec.Multiplexed())
	if err != nil {
		return nil, fmt.Errorf("failed to create superuser: %w", err)
	}

	// Authenticate and get token
	token, err := authenticate(uri, testSuperuserEmail, testSuperuserPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate: %w", err)
	}

	return &PocketBaseContainer{
		Container: container,
		URI:       uri,
		Token:     token,
	}, nil
}

func authenticate(uri, email, password string) (string, error) {
	payload := map[string]string{
		"identity": email,
		"password": password,
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post(uri+"/api/collections/_superusers/auth-with-password", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("auth failed with status %d", resp.StatusCode)
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Token, nil
}

func (c *PocketBaseContainer) AuthenticatedGet(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.Token)
	return http.DefaultClient.Do(req)
}

func TestHabitsCollectionExists(t *testing.T) {
	ctx := context.Background()

	container, err := setupPocketBase(ctx, t)
	if err != nil {
		t.Fatalf("failed to start container: %v", err)
	}
	defer container.Terminate(ctx)

	resp, err := container.AuthenticatedGet(container.URI + "/api/collections/habits")
	if err != nil {
		t.Fatalf("failed to get habits collection: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestHabitsCollectionFields(t *testing.T) {
	ctx := context.Background()

	container, err := setupPocketBase(ctx, t)
	if err != nil {
		t.Fatalf("failed to start container: %v", err)
	}
	defer container.Terminate(ctx)

	resp, err := container.AuthenticatedGet(container.URI + "/api/collections/habits")
	if err != nil {
		t.Fatalf("failed to get habits collection: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var collection struct {
		Name   string `json:"name"`
		Fields []struct {
			Name     string   `json:"name"`
			Type     string   `json:"type"`
			Required bool     `json:"required"`
			Min      *int     `json:"min,omitempty"`
			Max      *int     `json:"max,omitempty"`
			Values   []string `json:"values,omitempty"`
		} `json:"fields"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&collection); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if collection.Name != "habits" {
		t.Errorf("expected collection name 'habits', got '%s'", collection.Name)
	}

	expectedFields := map[string]struct {
		Type     string
		Required bool
	}{
		"name":        {Type: "text", Required: true},
		"description": {Type: "text", Required: false},
		"type":        {Type: "select", Required: true},
		"points":      {Type: "number", Required: false},
		"userId":      {Type: "relation", Required: true},
	}

	fieldMap := make(map[string]struct {
		Type     string
		Required bool
	})
	for _, f := range collection.Fields {
		fieldMap[f.Name] = struct {
			Type     string
			Required bool
		}{Type: f.Type, Required: f.Required}
	}

	for name, expected := range expectedFields {
		actual, exists := fieldMap[name]
		if !exists {
			t.Errorf("expected field '%s' not found", name)
			continue
		}
		if actual.Type != expected.Type {
			t.Errorf("field '%s': expected type '%s', got '%s'", name, expected.Type, actual.Type)
		}
		if actual.Required != expected.Required {
			t.Errorf("field '%s': expected required=%v, got %v", name, expected.Required, actual.Required)
		}
	}
}
