package controllers

import "golang.org/x/net/context"

type Controller interface {
	Start(ctx context.Context) error
}
