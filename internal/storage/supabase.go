package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type SupabasePhotoStorage struct {
	baseURL    string
	serviceKey string
	bucket     string
	client     *http.Client
}

func NewSupabasePhotoStorage(projectURL, serviceKey, bucket string) (*SupabasePhotoStorage, error) {
	projectURL = strings.TrimRight(strings.TrimSpace(projectURL), "/")
	if projectURL == "" || serviceKey == "" || bucket == "" {
		return nil, fmt.Errorf("SUPABASE_URL, SUPABASE_SERVICE_ROLE_KEY, and photo bucket are required")
	}
	if _, err := url.ParseRequestURI(projectURL); err != nil {
		return nil, fmt.Errorf("invalid SUPABASE_URL: %w", err)
	}
	return &SupabasePhotoStorage{baseURL: projectURL, serviceKey: serviceKey, bucket: bucket, client: &http.Client{Timeout: 20 * time.Second}}, nil
}

func (s *SupabasePhotoStorage) Put(ctx context.Context, objectPath, contentType string, body io.Reader) error {
	return s.do(ctx, http.MethodPost, s.objectURL(objectPath, false), contentType, body, map[string]string{"x-upsert": "false"}, nil)
}

func (s *SupabasePhotoStorage) Open(ctx context.Context, objectPath string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.objectURL(objectPath, false), nil)
	if err != nil {
		return nil, err
	}
	s.setAuth(req)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, responseError(resp)
	}
	return resp.Body, nil
}

func (s *SupabasePhotoStorage) SignedURL(ctx context.Context, objectPath string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		return "", fmt.Errorf("signed URL TTL must be positive")
	}
	body, err := json.Marshal(map[string]int64{"expiresIn": int64(ttl / time.Second)})
	if err != nil {
		return "", err
	}
	var result struct {
		SignedURL string `json:"signedURL"`
	}
	if err := s.do(ctx, http.MethodPost, s.objectURL(objectPath, true), "application/json", bytes.NewReader(body), nil, &result); err != nil {
		return "", err
	}
	if result.SignedURL == "" {
		return "", fmt.Errorf("Supabase Storage returned an empty signed URL")
	}
	if strings.HasPrefix(result.SignedURL, "http://") || strings.HasPrefix(result.SignedURL, "https://") {
		return result.SignedURL, nil
	}
	return s.baseURL + "/storage/v1" + ensureLeadingSlash(result.SignedURL), nil
}

func (s *SupabasePhotoStorage) Delete(ctx context.Context, objectPath string) error {
	body, err := json.Marshal(map[string][]string{"prefixes": []string{objectPath}})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, s.baseURL+"/storage/v1/object/"+url.PathEscape(s.bucket), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	s.setAuth(req)
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if (resp.StatusCode >= 200 && resp.StatusCode < 300) || resp.StatusCode == http.StatusNotFound {
		return nil
	}
	return responseError(resp)
}

func (s *SupabasePhotoStorage) do(ctx context.Context, method, endpoint, contentType string, body io.Reader, headers map[string]string, target any) error {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return err
	}
	s.setAuth(req)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return responseError(resp)
	}
	if target != nil {
		return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(target)
	}
	return nil
}

func (s *SupabasePhotoStorage) setAuth(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+s.serviceKey)
	req.Header.Set("apikey", s.serviceKey)
}

func (s *SupabasePhotoStorage) objectURL(objectPath string, signed bool) string {
	segments := strings.Split(strings.Trim(objectPath, "/"), "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	prefix := "/storage/v1/object/"
	if signed {
		prefix += "sign/"
	}
	return s.baseURL + prefix + url.PathEscape(s.bucket) + "/" + strings.Join(segments, "/")
}

func responseError(resp *http.Response) error {
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("Supabase Storage returned %s: %s", resp.Status, strings.TrimSpace(string(data)))
}

func ensureLeadingSlash(path string) string {
	if strings.HasPrefix(path, "/") {
		return path
	}
	return "/" + path
}
