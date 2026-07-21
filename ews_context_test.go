package ews

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Test_Client_SendAndReceiveContext_cancellation проверяет, что отмена
// контекста прерывает HTTP-запрос со стороны клиента, а не ждёт ответа
// сервера.
func Test_Client_SendAndReceiveContext_cancellation(t *testing.T) {
	// Сервер, который задерживает ответ на 2 секунды — дольше любой
	// разумной отмены. Handler выходит по r.Context().Done() (обрыв
	// соединения клиентом) или по внутреннему таймауту, чтобы не
	// блокировать srv.Close().
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "u", "p", &Config{})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := c.SendAndReceiveContext(ctx, []byte("<x/>"))
	elapsed := time.Since(start)

	assert.Error(t, err)
	// Не должно быть *ews.HTTPError — это контекстная/транспортная ошибка.
	var httpErr *HTTPError
	assert.False(t, errors.As(err, &httpErr), "expected context error, got HTTPError: %v", err)

	// Запрос должен оборваться существенно быстрее 2-секундного ответа
	// сервера. Берём границу в 1 секунду.
	if elapsed >= time.Second {
		t.Fatalf("context cancellation should abort request promptly; took %v", elapsed)
	}
}

// Test_Client_SendAndReceiveContext_pre_cancelled_ctx — заранее
// отменённый контекст должен возвращать ошибку до отправки HTTP.
func Test_Client_SendAndReceiveContext_pre_cancelled_ctx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be hit when ctx is already cancelled")
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "u", "p", &Config{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.SendAndReceiveContext(ctx, []byte("<x/>"))
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
}

// Test_Client_SendAndReceive_delegates_to_context — backward-compat:
// существующий метод без ctx должен работать через context.Background().
func Test_Client_SendAndReceive_delegates_to_context(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "u", "p", &Config{})
	resp, err := c.SendAndReceive([]byte("<x/>"))
	assert.NoError(t, err)
	assert.Equal(t, []byte("ok"), resp)
	assert.True(t, called)
}
