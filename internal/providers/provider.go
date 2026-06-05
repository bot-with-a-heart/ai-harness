package providers

import "context"

type Model struct {
	ID      string
	Object  string
	Created int64
	OwnedBy string
}

type AskRequest struct {
	Model  string
	Prompt string
}

type AskResponse struct {
	Model   string
	Content string
}

type Provider interface {
	Name() string
	Health(context.Context) error
	ListModels(context.Context) ([]Model, error)
	Ask(context.Context, AskRequest) (AskResponse, error)
}
