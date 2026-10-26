package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/inox/inox/backend/internal/api/handler"
	"github.com/inox/inox/backend/internal/auth"
)

func TestRouter_CommunityRoutesRequireAuth(t *testing.T) {
	router := NewRouter(nil, auth.NewAuthService(nil, nil), nil, nil, nil, nil, nil,
		nil, nil, nil, nil, handler.NewCommunityHandler(nil), nil, nil)

	for _, tc := range []struct{ method, path string }{
		{"GET", "/api/v1/community/posts"},
		{"GET", "/api/v1/admin/community/posts"},
		{"POST", "/api/v1/admin/community/posts"},
		{"PATCH", "/api/v1/admin/community/posts/7d1c6f4e-8c1a-4a55-9a43-2b1f0f2b9e10"},
		{"DELETE", "/api/v1/admin/community/posts/7d1c6f4e-8c1a-4a55-9a43-2b1f0f2b9e10"},
	} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: status = %d, want 401", tc.method, tc.path, rec.Code)
		}
	}
}
