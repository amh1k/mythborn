package activities

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/amh1k/mythborn/internal/domain"
)

type SupabaseAuthAdmin struct {
	baseURL    string
	serviceKey string
	client     *http.Client
}

func NewSupabaseAuthAdmin(projectURL, serviceKey string) (*SupabaseAuthAdmin, error) {
	projectURL = strings.TrimRight(strings.TrimSpace(projectURL), "/")
	if projectURL == "" || strings.TrimSpace(serviceKey) == "" {
		return nil, fmt.Errorf("Supabase URL and service key are required")
	}
	if _, err := url.ParseRequestURI(projectURL); err != nil {
		return nil, fmt.Errorf("invalid Supabase project URL: %w", err)
	}
	return &SupabaseAuthAdmin{baseURL: projectURL, serviceKey: serviceKey, client: &http.Client{Timeout: 15 * time.Second}}, nil
}

func (s *SupabaseAuthAdmin) DeleteUser(ctx context.Context, accountID domain.ID) error {
	endpoint := s.baseURL + "/auth/v1/admin/users/" + url.PathEscape(string(accountID))
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.serviceKey)
	req.Header.Set("apikey", s.serviceKey)
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 || resp.StatusCode == http.StatusNotFound {
		return nil
	}
	return fmt.Errorf("Supabase Auth user deletion returned %s", resp.Status)
}
