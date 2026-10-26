package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/inox/inox/backend/internal/api/handler"
	"github.com/inox/inox/backend/internal/auth"
)

func TestRouter_AdminRoomRoutesRequireAuth(t *testing.T) {
	router := NewRouter(nil, auth.NewAuthService(nil, nil), handler.NewRoomHandler(nil), nil, nil, nil, &handler.AdminHandler{},
		nil, nil, nil, nil, nil, nil, nil)

	for _, tc := range []struct{ method, path string }{
		{"GET", "/api/v1/admin/rooms"},
		{"DELETE", "/api/v1/admin/rooms/7d1c6f4e-8c1a-4a55-9a43-2b1f0f2b9e10"},
		{"POST", "/api/v1/admin/rooms/delete"},
	} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: status = %d, want 401", tc.method, tc.path, rec.Code)
		}
	}
}
