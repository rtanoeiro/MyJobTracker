package handlers

import (
	"context"
	"job-applications/internal/middlewares"
	"job-applications/internal/session"
	"job-applications/internal/templates"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
)

func newMockRenderer(shouldFail bool) templates.Renderer {
	return &templates.MockRenderer{ShouldFail: shouldFail}
}

func formRequest(method, target string, values url.Values) *http.Request {
	req, _ := http.NewRequest(method, target, strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func withUserID(req *http.Request, userID int32) *http.Request {
	ctx := context.WithValue(req.Context(), middlewares.UserIDContextKey, userID)
	return req.WithContext(ctx)
}

func withSession(req *http.Request, sess *session.Session) *http.Request {
	ctx := context.WithValue(req.Context(), middlewares.SessionContextKey, sess)
	ctx = context.WithValue(ctx, middlewares.UserIDContextKey, sess.UserID)
	return req.WithContext(ctx)
}

func requestWithURLParam(method, path, paramName, paramValue string) *http.Request {
	req, _ := http.NewRequest(method, path, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(paramName, paramValue)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func formRequestWithURLParam(method, path string, values url.Values, paramName, paramValue string) *http.Request {
	req := formRequest(method, path, values)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(paramName, paramValue)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}
