package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/agiledigital-labs/terraform-provider-pingone-aic/internal/oauth2client"
)

func (c *Client) OAuth2ClientsPath(realm string) string {
	return c.RealmPath(realm) + "/realm-config/agents/OAuth2Client"
}

func validateOAuth2ClientID(id string) error {
	if id == "" {
		return fmt.Errorf("oauth2 client id is empty")
	}
	if strings.ContainsAny(id, "/\\") {
		return fmt.Errorf("oauth2 client id %q contains a path separator", id)
	}
	return nil
}

func (c *Client) ListOAuth2Clients(ctx context.Context, realm string) ([]string, error) {
	var ids []string
	cookie := ""
	for {
		q := url.Values{
			"_queryFilter": {"true"},
			"_fields":      {"_id"},
			"_pageSize":    {"1000"},
		}
		if cookie != "" {
			q.Set("_pagedResultsCookie", cookie)
		}
		reqPath := c.OAuth2ClientsPath(realm) + "?" + q.Encode()
		req, err := c.NewRequestVersion(ctx, http.MethodGet, reqPath, OAuth2APIVersion, nil)
		if err != nil {
			return nil, err
		}
		var body struct {
			Result             []map[string]any `json:"result"`
			PagedResultsCookie string           `json:"pagedResultsCookie"`
		}
		if err := c.Do(req, http.StatusOK, &body); err != nil {
			return nil, err
		}
		for _, row := range body.Result {
			if id, _ := row["_id"].(string); id != "" {
				ids = append(ids, id)
			}
		}
		if body.PagedResultsCookie == "" {
			break
		}
		cookie = body.PagedResultsCookie
	}
	return ids, nil
}

func (c *Client) GetOAuth2Client(ctx context.Context, realm, id string) (map[string]any, error) {
	if err := validateOAuth2ClientID(id); err != nil {
		return nil, err
	}
	reqPath := c.OAuth2ClientsPath(realm) + "/" + url.PathEscape(id)
	req, err := c.NewRequestVersion(ctx, http.MethodGet, reqPath, OAuth2APIVersion, nil)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := c.Do(req, http.StatusOK, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func (c *Client) OAuth2ClientTemplate(ctx context.Context, realm string) (map[string]any, error) {
	reqPath := c.OAuth2ClientsPath(realm) + "?_action=template"
	req, err := c.NewRequestVersion(ctx, http.MethodPost, reqPath, OAuth2APIVersion, map[string]any{})
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := c.Do(req, http.StatusOK, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func (c *Client) PutOAuth2Client(ctx context.Context, realm, id string, body map[string]any) (map[string]any, error) {
	if err := validateOAuth2ClientID(id); err != nil {
		return nil, err
	}
	reqPath := c.OAuth2ClientsPath(realm) + "/" + url.PathEscape(id)
	req, err := c.NewRequestVersion(ctx, http.MethodPut, reqPath, OAuth2APIVersion, oauth2client.SanitizeWrite(body))
	if err != nil {
		return nil, err
	}
	status, raw, err := c.DoStatus(req)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return nil, &APIError{Status: status, Body: string(raw), Method: http.MethodPut, URL: req.URL.String()}
	}
	// Re-read: create echo wraps fields and may omit write-only secrets.
	return c.GetOAuth2Client(ctx, realm, id)
}

func (c *Client) DeleteOAuth2Client(ctx context.Context, realm, id string) error {
	if err := validateOAuth2ClientID(id); err != nil {
		return err
	}
	reqPath := c.OAuth2ClientsPath(realm) + "/" + url.PathEscape(id)
	req, err := c.NewRequestVersion(ctx, http.MethodDelete, reqPath, OAuth2APIVersion, nil)
	if err != nil {
		return err
	}
	status, raw, err := c.DoStatus(req)
	if err != nil {
		return err
	}
	if status == http.StatusNotFound {
		return nil
	}
	if status != http.StatusOK {
		return &APIError{Status: status, Body: string(raw), Method: http.MethodDelete, URL: req.URL.String()}
	}
	return nil
}
