package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type UnifiClient struct {
	baseURL string
	token   string
	client  *http.Client
}

type siteResponse struct {
	Data []site `json:"data"`
}

type site struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	SiteName string `json:"siteName"`
}

type dnsPolicy struct {
	ID            string `json:"id"`
	Type          string `json:"type"`
	Enabled       bool   `json:"enabled"`
	Domain        string `json:"domain"`
	IPv4Address   string `json:"ipv4Address"`
	CanonicalName string `json:"canonicalName"`
	CNAME         string `json:"cname"`
	Text          string `json:"text"`
	TXT           string `json:"txt"`
	Value         string `json:"value"`
	TTLSeconds    int    `json:"ttlSeconds"`
	ServerDomain  string `json:"serverDomain"`
	Service       string `json:"service"`
	Protocol      string `json:"protocol"`
	Port          int    `json:"port"`
	Priority      int    `json:"priority"`
	Weight        int    `json:"weight"`
	Metadata      struct {
		Origin string `json:"origin"`
	} `json:"metadata"`
}

type listPoliciesResponse struct {
	Offset     int         `json:"offset"`
	Limit      int         `json:"limit"`
	Count      int         `json:"count"`
	TotalCount int         `json:"totalCount"`
	Data       []dnsPolicy `json:"data"`
}

func NewUnifiClient(baseURL, token string, insecureTLS bool) *UnifiClient {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: insecureTLS}

	return &UnifiClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}
}

func (c *UnifiClient) getSiteIDByName(ctx context.Context, siteName string) (string, error) {
	body, err := c.doRequest(ctx, http.MethodGet, "/v1/sites", nil)
	if err != nil {
		return "", err
	}

	var response siteResponse
	if err := json.Unmarshal(body, &response); err == nil && len(response.Data) > 0 {
		return matchSiteID(response.Data, siteName)
	}

	var sites []site
	if err := json.Unmarshal(body, &sites); err != nil {
		return "", fmt.Errorf("parse sites response: %w", err)
	}
	return matchSiteID(sites, siteName)
}

func matchSiteID(sites []site, siteName string) (string, error) {
	for _, s := range sites {
		if strings.EqualFold(s.Name, siteName) || strings.EqualFold(s.SiteName, siteName) {
			return s.ID, nil
		}
	}
	return "", fmt.Errorf("site %q not found", siteName)
}

func (c *UnifiClient) listDNSPolicies(ctx context.Context, siteID string) ([]dnsPolicy, error) {
	all := make([]dnsPolicy, 0)
	offset := 0
	limit := 200
	for {
		path := fmt.Sprintf("/v1/sites/%s/dns/policies?offset=%d&limit=%d", url.PathEscape(siteID), offset, limit)
		body, err := c.doRequest(ctx, http.MethodGet, path, nil)
		if err != nil {
			return nil, err
		}

		var response listPoliciesResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return nil, fmt.Errorf("parse dns policies response: %w", err)
		}

		all = append(all, response.Data...)
		offset += response.Count
		if offset >= response.TotalCount || response.Count == 0 {
			break
		}
	}

	return all, nil
}

func (c *UnifiClient) createDNSPolicy(ctx context.Context, siteID string, record DNSRecord) error {
	payload := map[string]any{
		"type":    record.Type,
		"enabled": true,
		"domain":  record.Domain,
	}
	switch record.Type {
		case recordTypeA:
			payload["ipv4Address"] = record.Value
			payload["ttlSeconds"] = record.TTL
		case recordTypeCNAME:
			payload["cname"] = record.Value
			payload["ttlSeconds"] = record.TTL
		case recordTypeSRV:
			payload["serverDomain"] = record.SrvTarget
			payload["service"] = record.SrvService
			payload["protocol"] = record.SrvProtocol
			payload["port"] = record.SrvPort
			payload["priority"] = record.SrvPriority
			payload["weight"] = record.SrvWeight
		default:
			payload["text"] = record.Value
	}

	path := fmt.Sprintf("/v1/sites/%s/dns/policies", url.PathEscape(siteID))
	_, err := c.doRequest(ctx, http.MethodPost, path, payload)
	return err
}

func (c *UnifiClient) updateDNSPolicy(ctx context.Context, siteID, policyID string, record DNSRecord) error {
	payload := map[string]any{
		"type":    record.Type,
		"enabled": true,
		"domain":  record.Domain,
	}
	switch record.Type {
		case recordTypeA:
			payload["ipv4Address"] = record.Value
			payload["ttlSeconds"] = record.TTL
		case recordTypeCNAME:
			payload["cname"] = record.Value
			payload["ttlSeconds"] = record.TTL
		case recordTypeSRV:
			payload["serverDomain"] = record.SrvTarget
			payload["service"] = record.SrvService
			payload["protocol"] = record.SrvProtocol
			payload["port"] = record.SrvPort
			payload["priority"] = record.SrvPriority
			payload["weight"] = record.SrvWeight
		default:
			payload["text"] = record.Value
	}

	path := fmt.Sprintf("/v1/sites/%s/dns/policies/%s", url.PathEscape(siteID), url.PathEscape(policyID))
	_, err := c.doRequest(ctx, http.MethodPut, path, payload)
	return err
}

func (c *UnifiClient) deleteDNSPolicy(ctx context.Context, siteID, policyID string) error {
	path := fmt.Sprintf("/v1/sites/%s/dns/policies/%s", url.PathEscape(siteID), url.PathEscape(policyID))
	_, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	return err
}

func (c *UnifiClient) doRequest(ctx context.Context, method, path string, payload any) ([]byte, error) {
	endpoint := c.baseURL + "/proxy/network/integration" + path
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-KEY", c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unifi api %s %s: status %d: %s", method, endpoint, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return respBody, nil
}
