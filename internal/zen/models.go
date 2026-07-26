package zen

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const modelsPath = "models"

// Model is one model entry supplied by Zen's OpenAI-compatible list endpoint.
type Model struct {
	ID string `json:"id"`
}

// ModelsSchemaError reports a malformed or incompatible models response without
// retaining the upstream response body.
type ModelsSchemaError struct {
	cause error
}

func (err *ModelsSchemaError) Error() string { return "zen: invalid models response schema" }

func (err *ModelsSchemaError) Unwrap() error { return err.cause }

type modelsResponse struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

// Models fetches and parses Zen's OpenAI-compatible model list.
func (client *Client) Models(ctx context.Context) (_ []Model, err error) {
	endpoint := client.baseURL.JoinPath(modelsPath)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create Zen models request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer public")

	response, err := client.httpClient.Do(request)
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return nil, fmt.Errorf("Zen models request cancelled: %w", ctx.Err())
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, &TimeoutError{cause: err}
		}
		return nil, fmt.Errorf("send Zen models request: %w", err)
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close Zen models response: %w", closeErr)
		}
	}()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, &StatusError{StatusCode: response.StatusCode}
	}

	var payload modelsResponse
	decoder := json.NewDecoder(response.Body)
	if decodeErr := decoder.Decode(&payload); decodeErr != nil {
		return nil, &ModelsSchemaError{cause: decodeErr}
	}
	if decodeErr := rejectTrailingJSON(decoder); decodeErr != nil {
		return nil, &ModelsSchemaError{cause: decodeErr}
	}
	if payload.Object != "list" || payload.Data == nil {
		return nil, &ModelsSchemaError{}
	}
	for _, model := range payload.Data {
		if strings.TrimSpace(model.ID) == "" {
			return nil, &ModelsSchemaError{}
		}
	}
	return payload.Data, nil
}

func rejectTrailingJSON(decoder *json.Decoder) error {
	var extra struct{}
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errors.New("multiple JSON values")
	}
	return err
}
